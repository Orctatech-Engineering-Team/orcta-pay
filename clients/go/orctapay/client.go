// Package orctapay is a thin Go client for the Orcta Pay API.
//
// See api/openapi.yaml for the wire contract and
// orcta-go-docs/PAYMENTS_SERVICE_DESIGN.md for reference and idempotency rules.
package orctapay

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/orctatech/orcta-pay/internal/money"
)

// DefaultBaseURL is used for local development.
const DefaultBaseURL = "http://localhost:8080"

// Errors.
var (
	ErrRequestFailed  = errors.New("orctapay: request failed")
	ErrUnauthorized   = errors.New("orctapay: unauthorized")
	ErrNotFound       = errors.New("orctapay: not found")
	ErrInvalidRequest = errors.New("orctapay: invalid request")
)

// APIError carries the API error envelope details for a non-2xx response.
type APIError struct {
	StatusCode int
	Code       string
	Message    string
}

func (e *APIError) Error() string {
	return fmt.Sprintf("orctapay: %s: %s", e.Code, e.Message)
}

// Client is a thin wrapper over the Orcta Pay HTTP API.
type Client struct {
	baseURL    string
	apiKey     string
	httpClient *http.Client
}

// Option configures a Client.
type Option func(*Client)

// WithHTTPClient replaces the default 10s client.
func WithHTTPClient(c *http.Client) Option {
	return func(cl *Client) {
		if c != nil {
			cl.httpClient = c
		}
	}
}

// WithBaseURL overrides the base URL (useful in tests with httptest).
func WithBaseURL(url string) Option {
	return func(c *Client) {
		if url != "" {
			c.baseURL = strings.TrimRight(url, "/")
		}
	}
}

// NewClient builds a Client.
// baseURL defaults to ORCTA_PAY_URL env or http://localhost:8080 for local.
func NewClient(baseURL, apiKey string, opts ...Option) *Client {
	if baseURL == "" {
		baseURL = os.Getenv("ORCTA_PAY_URL")
	}
	if baseURL == "" {
		baseURL = DefaultBaseURL
	}
	baseURL = strings.TrimRight(baseURL, "/")
	c := &Client{
		baseURL:    baseURL,
		apiKey:     apiKey,
		httpClient: &http.Client{Timeout: 10 * time.Second},
	}
	for _, o := range opts {
		o(c)
	}
	if c.httpClient == nil {
		c.httpClient = &http.Client{Timeout: 10 * time.Second}
	}
	return c
}

// CreateChargeRequest is the input for POST /v1/charges.
type CreateChargeRequest struct {
	Product        string
	Amount         money.Money
	Wallet         string
	Phone          string
	IdempotencyKey string
	Metadata       map[string]any
}

// ChargeResult is the sealed result of a charge.
type ChargeResult interface{ isChargeResult() }

// ChargeSucceeded indicates the charge settled.
type ChargeSucceeded struct {
	Ref         string
	Gateway     string
	Amount      money.Money
	ExternalRef string
}

// ChargePending indicates the charge awaits confirmation.
type ChargePending struct {
	Ref         string
	Gateway     string
	ExternalRef string
}

// ChargeFailed indicates the charge failed synchronously.
type ChargeFailed struct {
	Reason string
}

func (ChargeSucceeded) isChargeResult() {}
func (ChargePending) isChargeResult()   {}
func (ChargeFailed) isChargeResult()    {}

// ChargeStatus is the result of GET /v1/charges/{ref}/status.
type ChargeStatus struct {
	Ref           string    `json:"ref"`
	Status        string    `json:"status"`
	Gateway       string    `json:"gateway"`
	AmountPesewas int64     `json:"amount_pesewas"`
	VerifiedAt    time.Time `json:"verified_at"`
}

// CreatePayoutRequest is the input for POST /v1/payouts.
type CreatePayoutRequest struct {
	Product        string
	Entries        []PayoutEntry
	IdempotencyKey string
}

// PayoutEntry is one recipient in a batch.
type PayoutEntry struct {
	Recipient string
	Amount    money.Money
}

// PayoutResult is the created batch.
type PayoutResult struct {
	BatchID      string `json:"batch_id"`
	Product      string `json:"product"`
	Status       string `json:"status"`
	TotalPesewas int64  `json:"total_pesewas"`
	Total        money.Money
	CreatedAt    time.Time `json:"created_at"`
}

// CreateAppRequest is the input for POST /v1/apps.
type CreateAppRequest struct {
	Name    string
	Product string
}

// CreateAppResponse is returned by POST /v1/apps.
// APIKey is shown exactly once — persist it immediately.
type CreateAppResponse struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	Product   string    `json:"product"`
	APIKey    string    `json:"api_key"`
	Prefix    string    `json:"api_key_prefix"`
	CreatedAt time.Time `json:"created_at"`
}

// App is a record from GET /v1/apps.
type App struct {
	ID         string     `json:"id"`
	Name       string     `json:"name"`
	Product    string     `json:"product"`
	Prefix     string     `json:"api_key_prefix"`
	CreatedAt  time.Time  `json:"created_at"`
	LastUsedAt *time.Time `json:"last_used_at,omitempty"`
	RevokedAt  *time.Time `json:"revoked_at,omitempty"`
	CreatedBy  string     `json:"created_by"`
}

// RotateAppKeyResponse is returned by POST /v1/apps/{id}/keys/rotate.
// APIKey is shown exactly once — persist it immediately.
type RotateAppKeyResponse struct {
	APIKey string `json:"api_key"`
	Prefix string `json:"api_key_prefix"`
}

// CreateApp creates an app and returns its API key (shown once).
func (c *Client) CreateApp(ctx context.Context, req CreateAppRequest) (CreateAppResponse, error) {
	if req.Name == "" {
		return CreateAppResponse{}, fmt.Errorf("%w: name is required", ErrInvalidRequest)
	}
	if req.Product == "" {
		return CreateAppResponse{}, fmt.Errorf("%w: product is required", ErrInvalidRequest)
	}
	body := map[string]any{"name": req.Name, "product": req.Product}
	var out CreateAppResponse
	if err := c.doJSON(ctx, http.MethodPost, "/v1/apps", body, &out); err != nil {
		return CreateAppResponse{}, err
	}
	return out, nil
}

// ListApps lists apps for the current organization.
func (c *Client) ListApps(ctx context.Context) ([]App, error) {
	var out []App
	if err := c.doJSON(ctx, http.MethodGet, "/v1/apps", nil, &out); err != nil {
		return nil, err
	}
	return out, nil
}

// RotateAppKey rotates an app's API key. The new key is shown once.
func (c *Client) RotateAppKey(ctx context.Context, appID string) (RotateAppKeyResponse, error) {
	if appID == "" {
		return RotateAppKeyResponse{}, fmt.Errorf("%w: appID is required", ErrInvalidRequest)
	}
	path := "/v1/apps/" + url.PathEscape(appID) + "/keys/rotate"
	var out RotateAppKeyResponse
	if err := c.doJSON(ctx, http.MethodPost, path, map[string]any{}, &out); err != nil {
		return RotateAppKeyResponse{}, err
	}
	return out, nil
}

// RotateKey is an alias for RotateAppKey.
func (c *Client) RotateKey(ctx context.Context, appID string) (RotateAppKeyResponse, error) {
	return c.RotateAppKey(ctx, appID)
}

// RevokeApp soft-deletes an app.
func (c *Client) RevokeApp(ctx context.Context, appID string) error {
	if appID == "" {
		return fmt.Errorf("%w: appID is required", ErrInvalidRequest)
	}
	path := "/v1/apps/" + url.PathEscape(appID)
	return c.doJSON(ctx, http.MethodDelete, path, nil, nil)
}

// CreateCharge initiates a charge.
func (c *Client) CreateCharge(ctx context.Context, req CreateChargeRequest) (ChargeResult, error) {
	key := req.IdempotencyKey
	if key == "" {
		product := req.Product
		if product == "" {
			product = "default"
		}
		key = buildReference(product, newULID())
	}
	currency := string(req.Amount.Currency())
	if currency == "" {
		currency = string(money.GHS)
	}
	body := map[string]any{
		"product":         req.Product,
		"amount_pesewas":  req.Amount.MinorUnits(),
		"currency":        currency,
		"idempotency_key": key,
	}
	wallet := req.Wallet
	if wallet == "" {
		wallet = req.Phone
	}
	if wallet != "" {
		body["wallet"] = wallet
		if req.Phone != "" && req.Phone != wallet {
			body["phone"] = req.Phone
		}
	}
	if req.Metadata != nil {
		body["metadata"] = req.Metadata
	}

	var resp chargeWire
	if err := c.doJSON(ctx, http.MethodPost, "/v1/charges", body, &resp); err != nil {
		// The service reports synchronous charge failure as 502 with code
		// "unavailable" and the gateway decline reason in the message. Surface
		// it as ChargeFailed rather than a transport error.
		var apiErr *APIError
		if errors.As(err, &apiErr) && apiErr.Code == "unavailable" {
			return ChargeFailed{Reason: apiErr.Message}, nil
		}
		return nil, err
	}
	// Map to sealed result based on status.
	switch resp.Status {
	case "pending":
		return ChargePending{Ref: resp.Ref, Gateway: resp.Gateway, ExternalRef: resp.ExternalRef}, nil
	case "succeeded":
		amt := money.New(resp.AmountPesewas, money.Currency(resp.Currency))
		return ChargeSucceeded{Ref: resp.Ref, Gateway: resp.Gateway, Amount: amt, ExternalRef: resp.ExternalRef}, nil
	case "failed":
		return ChargeFailed{Reason: resp.Status}, nil
	default:
		// Unknown status mapped to pending to preserve forward compatibility.
		if resp.Ref != "" {
			return ChargePending{Ref: resp.Ref, Gateway: resp.Gateway, ExternalRef: resp.ExternalRef}, nil
		}
		return ChargeFailed{Reason: "unknown status: " + resp.Status}, nil
	}
}

// GetChargeStatus fetches authoritative status.
func (c *Client) GetChargeStatus(ctx context.Context, ref string) (ChargeStatus, error) {
	if ref == "" {
		return ChargeStatus{}, fmt.Errorf("%w: ref is required", ErrInvalidRequest)
	}
	path := "/v1/charges/" + url.PathEscape(ref) + "/status"
	var out ChargeStatus
	if err := c.doJSON(ctx, http.MethodGet, path, nil, &out); err != nil {
		return ChargeStatus{}, err
	}
	return out, nil
}

// CreatePayout creates a payout batch.
func (c *Client) CreatePayout(ctx context.Context, req CreatePayoutRequest) (PayoutResult, error) {
	if req.Product == "" {
		return PayoutResult{}, fmt.Errorf("%w: product is required", ErrInvalidRequest)
	}
	if len(req.Entries) == 0 {
		return PayoutResult{}, fmt.Errorf("%w: entries required", ErrInvalidRequest)
	}
	entries := make([]map[string]any, 0, len(req.Entries))
	for _, e := range req.Entries {
		cur := string(e.Amount.Currency())
		if cur == "" {
			cur = string(money.GHS)
		}
		entries = append(entries, map[string]any{
			"recipient":      e.Recipient,
			"amount_pesewas": e.Amount.MinorUnits(),
			"currency":       cur,
		})
	}
	body := map[string]any{
		"product": req.Product,
		"entries": entries,
	}
	if req.IdempotencyKey != "" {
		body["idempotency_key"] = req.IdempotencyKey
	}
	var out PayoutResult
	if err := c.doJSON(ctx, http.MethodPost, "/v1/payouts", body, &out); err != nil {
		return PayoutResult{}, err
	}
	// Preserve money type from total_pesewas for caller convenience.
	if out.TotalPesewas != 0 {
		out.Total = money.New(out.TotalPesewas, money.GHS)
	}
	return out, nil
}

type chargeWire struct {
	Ref           string `json:"ref"`
	Product       string `json:"product"`
	Gateway       string `json:"gateway"`
	AmountPesewas int64  `json:"amount_pesewas"`
	Currency      string `json:"currency"`
	Status        string `json:"status"`
	ExternalRef   string `json:"external_ref"`
	CreatedAt     string `json:"created_at"`
}

func (c *Client) doJSON(ctx context.Context, method, path string, body any, out any) error {
	var bodyReader io.Reader
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			return fmt.Errorf("orctapay: marshal: %w", err)
		}
		bodyReader = bytes.NewReader(b)
	}
	url := c.baseURL + path
	req, err := http.NewRequestWithContext(ctx, method, url, bodyReader)
	if err != nil {
		return fmt.Errorf("orctapay: new request: %w", err)
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if c.apiKey != "" {
		req.Header.Set("Authorization", "Bearer "+c.apiKey)
	}
	req.Header.Set("Accept", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("orctapay: do: %w", err)
	}
	defer func() {
		_, _ = io.Copy(io.Discard, resp.Body)
		_ = resp.Body.Close()
	}()

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("orctapay: read: %w", err)
	}

	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		if out != nil && len(data) > 0 {
			if err := json.Unmarshal(data, out); err != nil {
				return fmt.Errorf("orctapay: decode: %w", err)
			}
		}
		return nil
	}

	// Decode error envelope if present.
	var env struct {
		Error struct {
			Code    string `json:"code"`
			Message string `json:"message"`
		} `json:"error"`
	}
	_ = json.Unmarshal(data, &env)
	msg := env.Error.Message
	if msg == "" {
		msg = strings.TrimSpace(string(data))
		if msg == "" {
			msg = resp.Status
		}
	}
	code := env.Error.Code
	if code == "" {
		switch {
		case resp.StatusCode == http.StatusUnauthorized:
			code = "unauthorized"
		case resp.StatusCode == http.StatusNotFound:
			code = "not_found"
		case resp.StatusCode == http.StatusBadRequest:
			code = "invalid_request"
		case resp.StatusCode >= 500:
			code = "internal_error"
		default:
			code = "unknown"
		}
	}
	apiErr := &APIError{StatusCode: resp.StatusCode, Code: code, Message: msg}

	switch {
	case resp.StatusCode == http.StatusUnauthorized:
		return errors.Join(ErrUnauthorized, apiErr)
	case resp.StatusCode == http.StatusNotFound:
		return errors.Join(ErrNotFound, apiErr)
	case resp.StatusCode == http.StatusBadRequest:
		return errors.Join(ErrInvalidRequest, apiErr)
	case resp.StatusCode >= 500:
		return errors.Join(ErrRequestFailed, apiErr)
	default:
		return apiErr
	}
}

// buildReference formats optd-{product}-{ulid}.
//
// No gateway segment: the gateway is chosen server-side per charge
// (hubtel / paystack / moolre, ranked by the router), so the client cannot
// know it up front. The authoritative gateway is returned on ChargeResult.
func buildReference(product, ulid string) string {
	return fmt.Sprintf("optd-%s-%s", product, ulid)
}

// newULID generates a 26-char Crockford Base32 ULID-like string.
func newULID() string {
	var entropy [10]byte
	_, _ = rand.Read(entropy[:])
	ms := uint64(time.Now().UTC().UnixMilli()) & 0xFFFFFFFFFF
	var id [16]byte
	binary.BigEndian.PutUint16(id[0:2], uint16(ms>>32))
	binary.BigEndian.PutUint32(id[2:6], uint32(ms))
	copy(id[6:], entropy[:])
	return encodeCrockford(id[:])
}

const crockford = "0123456789ABCDEFGHJKMNPQRSTVWXYZ"

func encodeCrockford(b []byte) string {
	out := make([]byte, 26)
	var buffer uint16
	var bitsLeft uint
	var idx int
	for _, by := range b {
		buffer = (buffer << 8) | uint16(by)
		bitsLeft += 8
		for bitsLeft >= 5 && idx < 26 {
			bitsLeft -= 5
			out[idx] = crockford[(buffer>>bitsLeft)&0x1F]
			idx++
		}
	}
	if idx < 26 {
		out[idx] = crockford[(buffer<<uint(5-bitsLeft))&0x1F]
	}
	return string(out)
}
