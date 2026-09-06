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

	"github.com/Orctatech-Engineering-Team/orcta-pay/internal/config"
	"github.com/Orctatech-Engineering-Team/orcta-pay/internal/money"
	"github.com/Orctatech-Engineering-Team/orcta-pay/internal/observability"
)

// PaystackAdapter implements AggregatorClient for Paystack.
type PaystackAdapter struct {
	cfg         config.PaystackConfig
	callbackURL string
	client      *http.Client
	logger      *slog.Logger
}

// NewPaystackAdapter returns an adapter for Paystack.
func NewPaystackAdapter(cfg config.PaystackConfig, opts ...PaystackOption) *PaystackAdapter {
	if cfg.BaseURL == "" {
		cfg.BaseURL = "https://api.paystack.co"
	}
	callbackURL := ""
	a := &PaystackAdapter{cfg: cfg, client: &http.Client{Timeout: 10 * time.Second}, logger: slog.Default(), callbackURL: callbackURL}
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

// PaystackOption configures PaystackAdapter.
type PaystackOption func(*PaystackAdapter)

// WithPaystackLogger sets the logger.
func WithPaystackLogger(l *slog.Logger) PaystackOption {
	return func(a *PaystackAdapter) { a.logger = l }
}

// WithPaystackClient overrides the HTTP client (for tests).
func WithPaystackClient(c *http.Client) PaystackOption {
	return func(a *PaystackAdapter) { a.client = c }
}

// WithPaystackBaseURL overrides the base URL (for tests).
func WithPaystackBaseURL(u string) PaystackOption {
	return func(a *PaystackAdapter) { a.cfg.BaseURL = strings.TrimRight(u, "/") }
}

// WithPaystackCallbackURL overrides the callback URL.
func WithPaystackCallbackURL(u string) PaystackOption {
	return func(a *PaystackAdapter) { a.callbackURL = u }
}

type paystackInitPayload struct {
	Email       string   `json:"email"`
	Amount      int64    `json:"amount"`
	Reference   string   `json:"reference"`
	CallbackURL string   `json:"callback_url,omitempty"`
	Currency    string   `json:"currency"`
	Channels    []string `json:"channels,omitempty"`
}

type paystackInitResponse struct {
	Status  bool   `json:"status"`
	Message string `json:"message"`
	Data    struct {
		AuthorizationURL string `json:"authorization_url"`
		AccessCode       string `json:"access_code"`
		Reference        string `json:"reference"`
	} `json:"data"`
}

type paystackVerifyResponse struct {
	Status  bool   `json:"status"`
	Message string `json:"message"`
	Data    struct {
		Status    string `json:"status"`
		Reference string `json:"reference"`
		Amount    int64  `json:"amount"`
		Currency  string `json:"currency"`
	} `json:"data"`
}

type paystackTransferRecipientPayload struct {
	Type          string `json:"type"`
	Name          string `json:"name"`
	AccountNumber string `json:"account_number"`
	BankCode      string `json:"bank_code"`
	Currency      string `json:"currency"`
}

type paystackTransferPayload struct {
	Source    string `json:"source"`
	Amount    int64  `json:"amount"`
	Recipient string `json:"recipient"`
	Reason    string `json:"reason"`
	Currency  string `json:"currency"`
	Reference string `json:"reference"`
}

// Initiate starts a charge via Paystack transaction/initialize.
func (a *PaystackAdapter) Initiate(ctx context.Context, req InitiateRequest) (InitiateResponse, error) {
	ctx, span := observability.StartSpan(ctx, "gateway.paystack.Initiate")
	defer span.End()

	if a.cfg.Disabled() {
		a.logger.InfoContext(ctx, "paystack not configured, skipping initiate", "reference", req.Reference)
		span.RecordError(ErrNotConfigured)
		return InitiateResponse{}, fmt.Errorf("paystack: %w", ErrNotConfigured)
	}
	callbackURL := a.callbackURL
	if callbackURL == "" {
		callbackURL = "https://api.pay.orctatech.com/webhooks/paystack"
	}
	email := fmt.Sprintf("customer+%s@orcta.pay", req.Reference)
	payload := paystackInitPayload{
		Email:       email,
		Amount:      req.Amount.MinorUnits(),
		Reference:   req.Reference,
		CallbackURL: callbackURL,
		Currency:    "GHS",
		Channels:    []string{"mobile_money", "card"},
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return InitiateResponse{}, fmt.Errorf("paystack: marshal: %w", err)
	}
	base := strings.TrimRight(a.cfg.BaseURL, "/")
	url := base + "/transaction/initialize"
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return InitiateResponse{}, fmt.Errorf("paystack: request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Accept", "application/json")
	httpReq.Header.Set("Authorization", "Bearer "+a.cfg.SecretKey)

	resp, err := a.client.Do(httpReq)
	if err != nil {
		span.RecordError(err)
		return InitiateResponse{}, fmt.Errorf("paystack: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()
	respBody, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))

	if resp.StatusCode >= 500 {
		span.RecordError(fmt.Errorf("paystack http %d", resp.StatusCode))
		return InitiateResponse{}, fmt.Errorf("paystack: http %d: %w", resp.StatusCode, ErrGatewayUnavailable)
	}
	if resp.StatusCode >= 400 {
		return InitiateResponse{}, fmt.Errorf("paystack: %s", extractPaystackReason(respBody, resp.StatusCode))
	}
	var pr paystackInitResponse
	if err := json.Unmarshal(respBody, &pr); err != nil {
		return InitiateResponse{ExternalRef: req.Reference, Status: "pending", RawRequest: body, RawResponse: respBody}, nil
	}
	if !pr.Status {
		reason := pr.Message
		if reason == "" {
			reason = "paystack declined"
		}
		return InitiateResponse{}, fmt.Errorf("paystack: %s", reason)
	}
	ref := pr.Data.Reference
	if ref == "" {
		ref = req.Reference
	}
	return InitiateResponse{ExternalRef: ref, Status: "pending", RawRequest: body, RawResponse: respBody}, nil
}

// Verify returns authoritative status via GET /transaction/verify/:reference.
func (a *PaystackAdapter) Verify(ctx context.Context, reference string) (VerifyResult, error) {
	ctx, span := observability.StartSpan(ctx, "gateway.paystack.Verify")
	defer span.End()

	if a.cfg.Disabled() {
		return VerifyResult{}, fmt.Errorf("paystack: %w", ErrNotConfigured)
	}
	base := strings.TrimRight(a.cfg.BaseURL, "/")
	url := base + "/transaction/verify/" + reference
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return VerifyResult{}, fmt.Errorf("paystack: request: %w", err)
	}
	httpReq.Header.Set("Authorization", "Bearer "+a.cfg.SecretKey)
	httpReq.Header.Set("Accept", "application/json")

	resp, err := a.client.Do(httpReq)
	if err != nil {
		span.RecordError(err)
		return VerifyResult{}, fmt.Errorf("paystack: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()
	respBody, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if resp.StatusCode >= 500 {
		return VerifyResult{}, fmt.Errorf("paystack: http %d: %w", resp.StatusCode, ErrGatewayUnavailable)
	}
	if resp.StatusCode >= 400 {
		return VerifyResult{}, fmt.Errorf("paystack: %s", extractPaystackReason(respBody, resp.StatusCode))
	}
	var pr paystackVerifyResponse
	if err := json.Unmarshal(respBody, &pr); err != nil {
		return VerifyResult{Reference: reference, Status: "pending", VerifiedAt: time.Now().UTC()}, nil
	}
	if !pr.Status {
		return VerifyResult{Reference: reference, Status: "failed", VerifiedAt: time.Now().UTC()}, nil
	}
	status := "pending"
	switch pr.Data.Status {
	case "success":
		status = "succeeded"
	case "failed", "abandoned":
		status = "failed"
	case "pending", "ongoing":
		status = "pending"
	}
	amount := money.New(pr.Data.Amount, money.GHS)
	return VerifyResult{Reference: reference, Status: status, Amount: amount, VerifiedAt: time.Now().UTC()}, nil
}

// Refund refunds a charge.
func (a *PaystackAdapter) Refund(ctx context.Context, reference string, amount money.Money) error {
	if a.cfg.Disabled() {
		return fmt.Errorf("paystack: %w", ErrNotConfigured)
	}
	a.logger.InfoContext(ctx, "paystack refund stub", "reference", reference)
	return nil
}

// Payout disburses funds via transfer recipient + transfer.
func (a *PaystackAdapter) Payout(ctx context.Context, recipient string, amount money.Money, reference string) error {
	ctx, span := observability.StartSpan(ctx, "gateway.paystack.Payout")
	defer span.End()

	if a.cfg.Disabled() {
		return fmt.Errorf("paystack: %w", ErrNotConfigured)
	}
	base := strings.TrimRight(a.cfg.BaseURL, "/")
	// Step 1: create recipient (simplified: assume bank code GHS, type mobile_money).
	recipientCode, err := a.createRecipient(ctx, base, recipient)
	if err != nil {
		return err
	}
	payload := paystackTransferPayload{
		Source:    "balance",
		Amount:    amount.MinorUnits(),
		Recipient: recipientCode,
		Reason:    reference,
		Currency:  "GHS",
		Reference: reference,
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("paystack: marshal: %w", err)
	}
	url := base + "/transfer"
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("paystack: request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", "Bearer "+a.cfg.SecretKey)
	httpReq.Header.Set("Accept", "application/json")

	resp, err := a.client.Do(httpReq)
	if err != nil {
		span.RecordError(err)
		return fmt.Errorf("paystack: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()
	respBody, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if resp.StatusCode >= 500 {
		span.RecordError(fmt.Errorf("paystack http %d", resp.StatusCode))
		return fmt.Errorf("paystack: http %d: %w", resp.StatusCode, ErrGatewayUnavailable)
	}
	if resp.StatusCode >= 400 {
		return fmt.Errorf("paystack: %s", extractPaystackReason(respBody, resp.StatusCode))
	}
	return nil
}

func (a *PaystackAdapter) createRecipient(ctx context.Context, base, recipient string) (string, error) {
	payload := paystackTransferRecipientPayload{
		Type:          "mobile_money",
		Name:          recipient,
		AccountNumber: recipient,
		BankCode:      "MTN",
		Currency:      "GHS",
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return "", fmt.Errorf("paystack: marshal recipient: %w", err)
	}
	url := base + "/transferrecipient"
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return "", fmt.Errorf("paystack: recipient request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", "Bearer "+a.cfg.SecretKey)
	httpReq.Header.Set("Accept", "application/json")

	resp, err := a.client.Do(httpReq)
	if err != nil {
		return "", fmt.Errorf("paystack: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()
	respBody, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if resp.StatusCode >= 500 {
		return "", fmt.Errorf("paystack: recipient http %d: %w", resp.StatusCode, ErrGatewayUnavailable)
	}
	if resp.StatusCode >= 400 {
		return "", fmt.Errorf("paystack: recipient: %s", extractPaystackReason(respBody, resp.StatusCode))
	}
	var out struct {
		Status bool `json:"status"`
		Data   struct {
			RecipientCode string `json:"recipient_code"`
		} `json:"data"`
		Message string `json:"message"`
	}
	if err := json.Unmarshal(respBody, &out); err != nil {
		return recipient, nil
	}
	if out.Data.RecipientCode != "" {
		return out.Data.RecipientCode, nil
	}
	return recipient, nil
}

func extractPaystackReason(body []byte, status int) string {
	var pr paystackInitResponse
	if err := json.Unmarshal(body, &pr); err == nil && pr.Message != "" {
		if len(pr.Message) > 256 {
			return pr.Message[:256]
		}
		return pr.Message
	}
	s := strings.TrimSpace(string(body))
	if len(s) > 256 {
		s = s[:256]
	}
	if s != "" {
		return s
	}
	return fmt.Sprintf("paystack status %d", status)
}
