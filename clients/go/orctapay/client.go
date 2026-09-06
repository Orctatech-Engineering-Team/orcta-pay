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
	CallbackURL    string
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

// CreateCharge initiates a charge.
func (c *Client) CreateCharge(ctx context.Context, req CreateChargeRequest) (ChargeResult, error) {
	key := req.IdempotencyKey
	if key == "" {
		product := req.Product
		if product == "" {
			product = "default"
		}
		key = buildReference(product, "hubtel", newULID())
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
	path := "/v1/charges/" + ref + "/status"
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

	switch resp.StatusCode {
	case http.StatusUnauthorized:
		return fmt.Errorf("%w: %s", ErrUnauthorized, msg)
	case http.StatusNotFound:
		return fmt.Errorf("%w: %s", ErrNotFound, msg)
	case http.StatusBadRequest:
		return fmt.Errorf("%w: %s", ErrInvalidRequest, msg)
	default:
		if resp.StatusCode >= 500 {
			return fmt.Errorf("%w: %s: %s", ErrRequestFailed, resp.Status, msg)
		}
		return fmt.Errorf("orctapay: %s: %s", resp.Status, msg)
	}
}

// buildReference formats optd-{product}-{gateway}-{ulid}.
func buildReference(product, gateway, ulid string) string {
	return fmt.Sprintf("optd-%s-%s-%s", product, gateway, ulid)
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
