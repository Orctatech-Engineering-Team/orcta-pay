package gateway

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/orctatech/orcta-pay/internal/config"
	"github.com/orctatech/orcta-pay/internal/money"
	"github.com/orctatech/orcta-pay/internal/observability"
)

// MoolreAdapter implements AggregatorClient for Moolre 2.0.
//
// Live base URL is https://api.moolre.com, sandbox is https://sandbox.moolre.com.
// Collections POST /open/transact/payment with X-API-USER + X-API-PUBKEY, field externalref.
// Transfers POST /open/transact/transfer with X-API-USER + X-API-KEY, field externalref.
// Amounts are decimal GHS strings ("18.00") derived from pesewas; do not send pesewas.
// Webhooks have no published HMAC; dedup via webhook_inbox is source of truth.
type MoolreAdapter struct {
	cfg         config.MoolreConfig
	callbackURL string
	client      *http.Client
	logger      *slog.Logger
}

// NewMoolreAdapter returns an adapter for Moolre.
func NewMoolreAdapter(cfg config.MoolreConfig, callbackBaseURL string, opts ...MoolreOption) *MoolreAdapter {
	if cfg.BaseURL == "" {
		cfg.BaseURL = "https://api.moolre.com"
	}
	callbackURL := ""
	if callbackBaseURL != "" {
		callbackURL = strings.TrimRight(callbackBaseURL, "/") + "/webhooks/moolre"
	}
	a := &MoolreAdapter{
		cfg:         cfg,
		callbackURL: callbackURL,
		client:      &http.Client{Timeout: 10 * time.Second},
		logger:      slog.Default(),
	}
	for _, opt := range opts {
		opt(a)
	}
	if a.client == nil {
		a.client = &http.Client{Timeout: 10 * time.Second}
	}
	if a.logger == nil {
		a.logger = slog.Default()
	}
	return a
}

// MoolreOption configures MoolreAdapter.
type MoolreOption func(*MoolreAdapter)

// WithMoolreLogger sets the logger.
func WithMoolreLogger(l *slog.Logger) MoolreOption {
	return func(a *MoolreAdapter) { a.logger = l }
}

// WithMoolreClient overrides the HTTP client (for tests).
func WithMoolreClient(c *http.Client) MoolreOption {
	return func(a *MoolreAdapter) { a.client = c }
}

// WithMoolreBaseURL overrides the API base URL (for tests).
func WithMoolreBaseURL(u string) MoolreOption {
	return func(a *MoolreAdapter) { a.cfg.BaseURL = strings.TrimRight(u, "/") }
}

type moolrePayPayload struct {
	Type          int    `json:"type"`
	Channel       string `json:"channel"`
	Currency      string `json:"currency"`
	Payer         string `json:"payer"`
	Amount        string `json:"amount"`
	ExternalRef   string `json:"externalref"`
	AccountNumber string `json:"accountnumber,omitempty"`
	Callback      string `json:"callback,omitempty"`
	Reference     string `json:"reference,omitempty"`
}

type moolreEnvelope struct {
	Status  any             `json:"status"`
	Code    string          `json:"code"`
	Message string          `json:"message"`
	Data    json.RawMessage `json:"data"`
}

type moolreLinkData struct {
	AuthorizationURL string `json:"authorization_url"`
	Reference        string `json:"reference"`
}

type moolreStatusPayload struct {
	Type          int    `json:"type"`
	IDType        int    `json:"idtype"`
	ID            string `json:"id"`
	AccountNumber string `json:"accountnumber,omitempty"`
}

type moolreStatusData struct {
	TxStatus      int    `json:"txstatus"`
	TxType        int    `json:"txtype"`
	Payer         string `json:"payer"`
	Payee         string `json:"payee"`
	Amount        string `json:"amount"`
	TransactionID string `json:"transactionid"`
	ExternalRef   string `json:"externalref"`
	ThirdPartyRef string `json:"thirdpartyref"`
	TS            string `json:"ts"`
}

type moolreTransferPayload struct {
	Type          int    `json:"type"`
	Channel       string `json:"channel"`
	Currency      string `json:"currency"`
	Amount        string `json:"amount"`
	Receiver      string `json:"receiver"`
	ExternalRef   string `json:"externalref"`
	Reference     string `json:"reference,omitempty"`
	AccountNumber string `json:"accountnumber,omitempty"`
	SubListID     string `json:"sublistid,omitempty"`
}

func moolreStatusSuccess(v any) bool {
	switch s := v.(type) {
	case float64:
		return s == 1
	case int:
		return s == 1
	case string:
		return s == "1"
	case json.Number:
		return s.String() == "1"
	default:
		return false
	}
}

func moolreAmount(m money.Money) string {
	return fmt.Sprintf("%.2f", float64(m.MinorUnits())/100)
}

func (a *MoolreAdapter) user() string {
	if a.cfg.APIUser != "" {
		return a.cfg.APIUser
	}
	if a.cfg.APIKey != "" {
		return a.cfg.APIKey
	}
	return a.cfg.APIPubKey
}

func (a *MoolreAdapter) pubKey() string {
	if a.cfg.APIPubKey != "" {
		return a.cfg.APIPubKey
	}
	return a.cfg.APIKey
}

func (a *MoolreAdapter) privKey() string {
	if a.cfg.APIKey != "" {
		return a.cfg.APIKey
	}
	return a.cfg.APIPubKey
}

// Initiate starts a charge via Moolre Mobile Money collection.
func (a *MoolreAdapter) Initiate(ctx context.Context, req InitiateRequest) (InitiateResponse, error) {
	ctx, span := observability.StartSpan(ctx, "gateway.moolre.Initiate")
	defer span.End()

	if a.cfg.Disabled() {
		a.logger.InfoContext(ctx, "moolre not configured, skipping initiate", "reference", req.Reference)
		span.RecordError(ErrNotConfigured)
		return InitiateResponse{}, fmt.Errorf("moolre: %w", ErrNotConfigured)
	}
	amountStr := moolreAmount(req.Amount)
	payer := req.Wallet
	if payer == "" {
		payer = "0240000000"
	}
	channel := "13"
	payload := moolrePayPayload{
		Type:          1,
		Channel:       channel,
		Currency:      "GHS",
		Payer:         payer,
		Amount:        amountStr,
		ExternalRef:   req.Reference,
		AccountNumber: a.cfg.AccountNumber,
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return InitiateResponse{}, fmt.Errorf("moolre: marshal: %w", err)
	}
	base := strings.TrimRight(a.cfg.BaseURL, "/")
	url := base + "/open/transact/payment"
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return InitiateResponse{}, fmt.Errorf("moolre: request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Accept", "application/json")
	httpReq.Header.Set("X-API-USER", a.user())
	httpReq.Header.Set("X-API-PUBKEY", a.pubKey())

	resp, err := a.client.Do(httpReq)
	if err != nil {
		span.RecordError(err)
		return InitiateResponse{}, fmt.Errorf("moolre: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()
	respBody, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))

	if resp.StatusCode >= 500 {
		span.RecordError(fmt.Errorf("moolre http %d", resp.StatusCode))
		return InitiateResponse{}, fmt.Errorf("moolre: http %d: %w", resp.StatusCode, ErrGatewayUnavailable)
	}
	if resp.StatusCode >= 400 {
		reason := extractMoolreReason(respBody, resp.StatusCode)
		return InitiateResponse{}, fmt.Errorf("moolre: %s", reason)
	}
	var env moolreEnvelope
	if err := json.Unmarshal(respBody, &env); err != nil {
		return InitiateResponse{ExternalRef: req.Reference, Status: "pending", RawRequest: body, RawResponse: respBody}, nil
	}
	if !moolreStatusSuccess(env.Status) {
		reason := env.Message
		if reason == "" {
			reason = env.Code
		}
		if reason == "" {
			reason = "moolre declined"
		}
		return InitiateResponse{}, fmt.Errorf("moolre: %s", reason)
	}
	externalRef := req.Reference
	var dataStr string
	if len(env.Data) > 0 {
		if err := json.Unmarshal(env.Data, &dataStr); err == nil && dataStr != "" {
			externalRef = dataStr
		} else {
			var link moolreLinkData
			if err := json.Unmarshal(env.Data, &link); err == nil && link.Reference != "" {
				externalRef = link.Reference
			}
		}
	}
	return InitiateResponse{ExternalRef: externalRef, Status: "pending", RawRequest: body, RawResponse: respBody}, nil
}

// Verify returns authoritative status via POST /open/transact/status.
func (a *MoolreAdapter) Verify(ctx context.Context, reference string) (VerifyResult, error) {
	ctx, span := observability.StartSpan(ctx, "gateway.moolre.Verify")
	defer span.End()

	if a.cfg.Disabled() {
		return VerifyResult{}, fmt.Errorf("moolre: %w", ErrNotConfigured)
	}
	payload := moolreStatusPayload{Type: 1, IDType: 1, ID: reference, AccountNumber: a.cfg.AccountNumber}
	body, err := json.Marshal(payload)
	if err != nil {
		return VerifyResult{}, fmt.Errorf("moolre: marshal: %w", err)
	}
	base := strings.TrimRight(a.cfg.BaseURL, "/")
	url := base + "/open/transact/status"
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return VerifyResult{}, fmt.Errorf("moolre: request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Accept", "application/json")
	httpReq.Header.Set("X-API-USER", a.user())
	// Status accepts either key; send both for compatibility.
	if k := a.privKey(); k != "" {
		httpReq.Header.Set("X-API-KEY", k)
	}
	if pk := a.pubKey(); pk != "" {
		httpReq.Header.Set("X-API-PUBKEY", pk)
	}

	resp, err := a.client.Do(httpReq)
	if err != nil {
		span.RecordError(err)
		return VerifyResult{}, fmt.Errorf("moolre: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()
	respBody, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if resp.StatusCode >= 500 {
		return VerifyResult{}, fmt.Errorf("moolre: http %d: %w", resp.StatusCode, ErrGatewayUnavailable)
	}
	if resp.StatusCode >= 400 {
		return VerifyResult{}, fmt.Errorf("moolre: http %d: %s", resp.StatusCode, extractMoolreReason(respBody, resp.StatusCode))
	}
	var env moolreEnvelope
	if err := json.Unmarshal(respBody, &env); err != nil {
		return VerifyResult{Reference: reference, Status: "pending", VerifiedAt: time.Now().UTC()}, nil
	}
	if !moolreStatusSuccess(env.Status) {
		return VerifyResult{Reference: reference, Status: "failed", VerifiedAt: time.Now().UTC()}, nil
	}
	var data moolreStatusData
	if err := json.Unmarshal(env.Data, &data); err != nil {
		return VerifyResult{Reference: reference, Status: "pending", VerifiedAt: time.Now().UTC()}, nil
	}
	status := "pending"
	switch data.TxStatus {
	case 1:
		status = "succeeded"
	case 2:
		status = "failed"
	}
	return VerifyResult{Reference: reference, Status: status, VerifiedAt: time.Now().UTC()}, nil
}

// Refund refunds a prior charge (no native endpoint, log and succeed if configured).
func (a *MoolreAdapter) Refund(ctx context.Context, reference string, amount money.Money) error {
	if a.cfg.Disabled() {
		return fmt.Errorf("moolre: %w", ErrNotConfigured)
	}
	a.logger.InfoContext(ctx, "moolre refund not natively supported, treating as no-op", "reference", reference)
	return nil
}

// Payout disburses to a recipient via POST /open/transact/transfer.
func (a *MoolreAdapter) Payout(ctx context.Context, recipient string, amount money.Money, reference string) error {
	ctx, span := observability.StartSpan(ctx, "gateway.moolre.Payout")
	defer span.End()

	if a.cfg.Disabled() {
		a.logger.InfoContext(ctx, "moolre not configured, skipping payout", "reference", reference)
		span.RecordError(ErrNotConfigured)
		return fmt.Errorf("moolre: %w", ErrNotConfigured)
	}
	amountStr := moolreAmount(amount)
	channel := "1"
	payload := moolreTransferPayload{
		Type:          1,
		Channel:       channel,
		Currency:      "GHS",
		Amount:        amountStr,
		Receiver:      recipient,
		ExternalRef:   reference,
		Reference:     reference,
		AccountNumber: a.cfg.AccountNumber,
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("moolre: marshal: %w", err)
	}
	base := strings.TrimRight(a.cfg.BaseURL, "/")
	url := base + "/open/transact/transfer"
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("moolre: request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Accept", "application/json")
	httpReq.Header.Set("X-API-USER", a.user())
	httpReq.Header.Set("X-API-KEY", a.privKey())

	resp, err := a.client.Do(httpReq)
	if err != nil {
		span.RecordError(err)
		return fmt.Errorf("moolre: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()
	respBody, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if resp.StatusCode >= 500 {
		span.RecordError(fmt.Errorf("moolre http %d", resp.StatusCode))
		return fmt.Errorf("moolre: http %d: %w", resp.StatusCode, ErrGatewayUnavailable)
	}
	if resp.StatusCode >= 400 {
		return fmt.Errorf("moolre: %s", extractMoolreReason(respBody, resp.StatusCode))
	}
	var env moolreEnvelope
	if err := json.Unmarshal(respBody, &env); err != nil {
		return nil
	}
	if !moolreStatusSuccess(env.Status) {
		return fmt.Errorf("moolre: payout failed: %s", env.Message)
	}
	return nil
}

// BulkPayoutEntry is a single recipient in a bulk disbursement.
type BulkPayoutEntry struct {
	Recipient string
	Amount    money.Money
	Reference string
}

// BulkPayoutResult holds per-recipient outcome.
type BulkPayoutResult struct {
	Reference string
	Err       error
}

// PayoutBulk disburses a batch by looping single transfers with distinct externalref.
// Moolre has no batch endpoint; dashboard CSV is the alternative. Each line uses
// a unique externalref which is the idempotency key. Caller should scope
// references to (vendor_id, batch_date) for idempotent resume per payments-design §7.
func (a *MoolreAdapter) PayoutBulk(ctx context.Context, entries []BulkPayoutEntry) []BulkPayoutResult {
	if a.cfg.Disabled() {
		results := make([]BulkPayoutResult, 0, len(entries))
		for _, e := range entries {
			a.logger.InfoContext(ctx, "moolre not configured, skipping bulk payout", "reference", e.Reference)
			results = append(results, BulkPayoutResult{Reference: e.Reference, Err: fmt.Errorf("moolre: %w", ErrNotConfigured)})
		}
		return results
	}
	results := make([]BulkPayoutResult, 0, len(entries))
	for _, e := range entries {
		err := a.Payout(ctx, e.Recipient, e.Amount, e.Reference)
		results = append(results, BulkPayoutResult{Reference: e.Reference, Err: err})
	}
	return results
}

func extractMoolreReason(body []byte, status int) string {
	var env moolreEnvelope
	if err := json.Unmarshal(body, &env); err == nil && env.Message != "" {
		if len(env.Message) > 256 {
			return env.Message[:256]
		}
		return env.Message
	}
	s := strings.TrimSpace(string(body))
	if len(s) > 256 {
		s = s[:256]
	}
	if s != "" {
		return s
	}
	return fmt.Sprintf("moolre status %d", status)
}
