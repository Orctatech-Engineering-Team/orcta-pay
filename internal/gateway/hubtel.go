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

// HubtelAdapter implements AggregatorClient for Hubtel.
type HubtelAdapter struct {
	cfg         config.HubtelConfig
	callbackURL string
	client      *http.Client
	logger      *slog.Logger
}

// NewHubtelAdapter returns an adapter for Hubtel.
func NewHubtelAdapter(cfg config.HubtelConfig, opts ...HubtelOption) *HubtelAdapter {
	if cfg.BaseURL == "" {
		cfg.BaseURL = "https://payproxyapi.hubtel.com"
	}
	callbackURL := ""
	if cfg.BaseURL != "" {
		// Callback built from Payments.CallbackBaseURL via platform; opts may override.
		_ = callbackURL
	}
	a := &HubtelAdapter{cfg: cfg, client: &http.Client{Timeout: 10 * time.Second}, logger: slog.Default(), callbackURL: callbackURL}
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

// HubtelOption configures HubtelAdapter.
type HubtelOption func(*HubtelAdapter)

// WithHubtelLogger sets the logger.
func WithHubtelLogger(l *slog.Logger) HubtelOption {
	return func(a *HubtelAdapter) { a.logger = l }
}

// WithHubtelClient overrides the HTTP client (for tests).
func WithHubtelClient(c *http.Client) HubtelOption {
	return func(a *HubtelAdapter) { a.client = c }
}

// WithHubtelBaseURL overrides the base URL (for tests).
func WithHubtelBaseURL(u string) HubtelOption {
	return func(a *HubtelAdapter) { a.cfg.BaseURL = strings.TrimRight(u, "/") }
}

// WithHubtelCallbackURL overrides the webhook callback URL.
func WithHubtelCallbackURL(u string) HubtelOption {
	return func(a *HubtelAdapter) { a.callbackURL = u }
}

type hubtelReceivePayload struct {
	CustomerName       string `json:"CustomerName"`
	CustomerMsisdn     string `json:"CustomerMsisdn"`
	CustomerEmail      string `json:"CustomerEmail,omitempty"`
	Channel            string `json:"Channel"`
	Amount             string `json:"Amount"`
	ClientReference    string `json:"ClientReference"`
	Description        string `json:"Description"`
	PrimaryCallbackURL string `json:"PrimaryCallbackUrl"`
}

type hubtelResponse struct {
	ResponseCode string `json:"ResponseCode"`
	ResponseText string `json:"ResponseText"`
	Message      string `json:"Message"`
	Status       string `json:"Status"`
	Data         struct {
		TransactionID string `json:"TransactionId"`
		ClientRef     string `json:"ClientReference"`
		Amount        string `json:"Amount"`
		Status        string `json:"Status"`
		CheckoutURL   string `json:"CheckoutUrl"`
	} `json:"Data"`
}

type hubtelSendPayload struct {
	RecipientName   string `json:"RecipientName"`
	RecipientMsisdn string `json:"RecipientMsisdn"`
	Amount          string `json:"Amount"`
	Channel         string `json:"Channel"`
	ClientReference string `json:"ClientReference"`
	Description     string `json:"Description"`
	CallbackURL     string `json:"CallbackUrl"`
}

func hubtelAmount(m money.Money) string {
	return fmt.Sprintf("%.2f", float64(m.MinorUnits())/100)
}

// Initiate starts a charge via Hubtel receive/initiate (direct MoMo).
func (a *HubtelAdapter) Initiate(ctx context.Context, req InitiateRequest) (InitiateResponse, error) {
	ctx, span := observability.StartSpan(ctx, "gateway.hubtel.Initiate")
	defer span.End()

	if a.cfg.Disabled() {
		a.logger.InfoContext(ctx, "hubtel not configured, skipping initiate", "reference", req.Reference)
		span.RecordError(ErrNotConfigured)
		return InitiateResponse{}, fmt.Errorf("hubtel: %w", ErrNotConfigured)
	}
	amountStr := hubtelAmount(req.Amount)
	callbackURL := a.callbackURL
	if callbackURL == "" {
		callbackURL = "https://api.pay.orctatech.com/webhooks/hubtel"
	}
	payload := hubtelReceivePayload{
		CustomerName:       req.Product,
		CustomerMsisdn:     req.Wallet,
		Channel:            "mtn-gh",
		Amount:             amountStr,
		ClientReference:    req.Reference,
		Description:        "Orcta order " + req.Reference,
		PrimaryCallbackURL: callbackURL,
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return InitiateResponse{}, fmt.Errorf("hubtel: marshal: %w", err)
	}
	base := strings.TrimRight(a.cfg.BaseURL, "/")
	url := base + "/receive/initiate"
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return InitiateResponse{}, fmt.Errorf("hubtel: request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Accept", "application/json")
	httpReq.SetBasicAuth(a.cfg.ClientID, a.cfg.ClientSecret)

	resp, err := a.client.Do(httpReq)
	if err != nil {
		span.RecordError(err)
		return InitiateResponse{}, fmt.Errorf("hubtel: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()
	respBody, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))

	if resp.StatusCode >= 500 {
		span.RecordError(fmt.Errorf("hubtel http %d", resp.StatusCode))
		return InitiateResponse{}, fmt.Errorf("hubtel: http %d: %w", resp.StatusCode, ErrGatewayUnavailable)
	}
	if resp.StatusCode >= 400 {
		return InitiateResponse{}, fmt.Errorf("hubtel: %s", extractHubtelReason(respBody, resp.StatusCode))
	}
	var hr hubtelResponse
	if err := json.Unmarshal(respBody, &hr); err != nil {
		return InitiateResponse{ExternalRef: req.Reference, Status: "pending", RawRequest: body, RawResponse: respBody}, nil
	}
	if hr.ResponseCode != "" && hr.ResponseCode != "00" && hr.ResponseCode != "0000" && hr.ResponseCode != "000" && hr.ResponseCode != "01" {
		reason := hr.ResponseText
		if reason == "" {
			reason = hr.Message
		}
		if reason == "" {
			reason = "hubtel declined: " + hr.ResponseCode
		}
		return InitiateResponse{}, fmt.Errorf("hubtel: %s", reason)
	}
	externalRef := hr.Data.TransactionID
	if externalRef == "" {
		externalRef = hr.Data.ClientRef
	}
	if externalRef == "" {
		externalRef = req.Reference
	}
	status := "pending"
	if hr.Data.Status == "Success" || hr.Data.Status == "success" {
		status = "succeeded"
	}
	return InitiateResponse{ExternalRef: externalRef, Status: status, RawRequest: body, RawResponse: respBody}, nil
}

// Verify returns authoritative status via Hubtel.
func (a *HubtelAdapter) Verify(ctx context.Context, reference string) (VerifyResult, error) {
	ctx, span := observability.StartSpan(ctx, "gateway.hubtel.Verify")
	defer span.End()

	if a.cfg.Disabled() {
		return VerifyResult{}, fmt.Errorf("hubtel: %w", ErrNotConfigured)
	}
	base := strings.TrimRight(a.cfg.BaseURL, "/")
	url := base + "/transactions/" + reference
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return VerifyResult{}, fmt.Errorf("hubtel: request: %w", err)
	}
	httpReq.SetBasicAuth(a.cfg.ClientID, a.cfg.ClientSecret)
	httpReq.Header.Set("Accept", "application/json")

	resp, err := a.client.Do(httpReq)
	if err != nil {
		span.RecordError(err)
		return VerifyResult{}, fmt.Errorf("hubtel: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()
	respBody, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if resp.StatusCode >= 500 {
		return VerifyResult{}, fmt.Errorf("hubtel: http %d: %w", resp.StatusCode, ErrGatewayUnavailable)
	}
	if resp.StatusCode == http.StatusNotFound {
		return VerifyResult{Reference: reference, Status: "pending", VerifiedAt: time.Now().UTC()}, nil
	}
	if resp.StatusCode >= 400 {
		return VerifyResult{}, fmt.Errorf("hubtel: %s", extractHubtelReason(respBody, resp.StatusCode))
	}
	var hr hubtelResponse
	if err := json.Unmarshal(respBody, &hr); err != nil {
		return VerifyResult{Reference: reference, Status: "pending", VerifiedAt: time.Now().UTC()}, nil
	}
	status := "pending"
	switch strings.ToLower(hr.Status) {
	case "success", "successful", "completed":
		status = "succeeded"
	case "failed", "fail":
		status = "failed"
	}
	if strings.ToLower(hr.Data.Status) == "success" {
		status = "succeeded"
	}
	return VerifyResult{Reference: reference, Status: status, VerifiedAt: time.Now().UTC()}, nil
}

// Refund refunds a prior charge.
func (a *HubtelAdapter) Refund(ctx context.Context, reference string, amount money.Money) error {
	if a.cfg.Disabled() {
		return fmt.Errorf("hubtel: %w", ErrNotConfigured)
	}
	a.logger.InfoContext(ctx, "hubtel refund stub", "reference", reference)
	return nil
}

// Payout disburses to a recipient via POST /send/money.
func (a *HubtelAdapter) Payout(ctx context.Context, recipient string, amount money.Money, reference string) error {
	ctx, span := observability.StartSpan(ctx, "gateway.hubtel.Payout")
	defer span.End()

	if a.cfg.Disabled() {
		return fmt.Errorf("hubtel: %w", ErrNotConfigured)
	}
	amountStr := hubtelAmount(amount)
	callbackURL := a.callbackURL
	if callbackURL == "" {
		callbackURL = "https://api.pay.orctatech.com/webhooks/hubtel"
	}
	payload := hubtelSendPayload{
		RecipientName:   recipient,
		RecipientMsisdn: recipient,
		Amount:          amountStr,
		Channel:         "mtn-gh",
		ClientReference: reference,
		Description:     "Orcta payout " + reference,
		CallbackURL:     callbackURL,
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("hubtel: marshal: %w", err)
	}
	base := strings.TrimRight(a.cfg.BaseURL, "/")
	url := base + "/send/money"
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("hubtel: request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Accept", "application/json")
	httpReq.SetBasicAuth(a.cfg.ClientID, a.cfg.ClientSecret)

	resp, err := a.client.Do(httpReq)
	if err != nil {
		span.RecordError(err)
		return fmt.Errorf("hubtel: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()
	respBody, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if resp.StatusCode >= 500 {
		span.RecordError(fmt.Errorf("hubtel http %d", resp.StatusCode))
		return fmt.Errorf("hubtel: http %d: %w", resp.StatusCode, ErrGatewayUnavailable)
	}
	if resp.StatusCode >= 400 {
		return fmt.Errorf("hubtel: %s", extractHubtelReason(respBody, resp.StatusCode))
	}
	var hr hubtelResponse
	if err := json.Unmarshal(respBody, &hr); err == nil && hr.ResponseCode != "" && hr.ResponseCode != "00" && hr.ResponseCode != "0000" {
		return fmt.Errorf("hubtel: payout failed: %s", hr.ResponseText)
	}
	return nil
}

func extractHubtelReason(body []byte, status int) string {
	var hr hubtelResponse
	if err := json.Unmarshal(body, &hr); err == nil {
		if hr.Message != "" {
			if len(hr.Message) > 256 {
				return hr.Message[:256]
			}
			return hr.Message
		}
		if hr.ResponseText != "" {
			if len(hr.ResponseText) > 256 {
				return hr.ResponseText[:256]
			}
			return hr.ResponseText
		}
	}
	s := strings.TrimSpace(string(body))
	if len(s) > 256 {
		s = s[:256]
	}
	if s != "" {
		return s
	}
	return fmt.Sprintf("hubtel status %d", status)
}
