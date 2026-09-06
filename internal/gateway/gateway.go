// Package gateway hides provider selection and per-gateway HTTP details behind AggregatorClient.
package gateway

import (
	"context"
	"errors"
	"time"

	"github.com/Orctatech-Engineering-Team/orcta-pay/internal/money"
)

// ResultRecorder records per-gateway outcomes for health tracking
// (implemented by the Valkey health store).
type ResultRecorder interface {
	RecordResult(ctx context.Context, gw Gateway, success bool) error
}

// Gateway identifies a provider.
type Gateway string

const (
	GatewayHubtel   Gateway = "hubtel"
	GatewayPaystack Gateway = "paystack"
	GatewayMoolre   Gateway = "moolre"
)

// Valid reports whether g is known.
func (g Gateway) Valid() bool {
	return g == GatewayHubtel || g == GatewayPaystack || g == GatewayMoolre
}

// ErrNotConfigured is returned when a gateway has no credentials.
var ErrNotConfigured = errors.New("gateway: not configured")

// ErrGatewayUnavailable is returned when the gateway is circuit-open or failing.
var ErrGatewayUnavailable = errors.New("gateway: unavailable")

// InitiateRequest is the normalized initiation payload.
type InitiateRequest struct {
	Reference      string
	Amount         money.Money
	Wallet         string
	Product        string
	IdempotencyKey string
}

// InitiateResponse is the normalized gateway response.
type InitiateResponse struct {
	ExternalRef string
	Status      string
	RawRequest  []byte
	RawResponse []byte
}

// VerifyResult is the authoritative status from GetTransactionStatus.
type VerifyResult struct {
	Reference  string
	Status     string // succeeded | pending | failed
	Amount     money.Money
	VerifiedAt time.Time
}

// AggregatorClient is the gateway seam. Initiate/Verify/Refund/Payout mirror ADR-034.
type AggregatorClient interface {
	Initiate(ctx context.Context, req InitiateRequest) (InitiateResponse, error)
	Verify(ctx context.Context, reference string) (VerifyResult, error)
	Refund(ctx context.Context, reference string, amount money.Money) error
	Payout(ctx context.Context, recipient string, amount money.Money, reference string) error
}

// ChargeResult is the sealed result of a charge attempt.
type ChargeResult interface{ isChargeResult() }

// ChargeSucceeded indicates the charge settled.
type ChargeSucceeded struct {
	ExternalRef string
	SettledAt   time.Time
}

// ChargePending indicates the charge is awaiting confirmation.
type ChargePending struct{ ExternalRef string }

// ChargeFailed indicates the charge failed.
type ChargeFailed struct{ Reason string }

func (ChargeSucceeded) isChargeResult() {}
func (ChargePending) isChargeResult()   {}
func (ChargeFailed) isChargeResult()    {}
