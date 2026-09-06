// Package charges hides collection lifecycle and reference generation.
package charges

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/orctatech/orcta-pay/internal/gateway"
	"github.com/orctatech/orcta-pay/internal/money"
	"github.com/orctatech/orcta-pay/internal/observability"
)

// Errors.
var (
	ErrInvalidRequest = errors.New("charges: invalid request")
	ErrNotFound       = errors.New("charges: not found")
)

// ChargeRequest is the normalized initiation input.
type ChargeRequest struct {
	Product        string
	Amount         money.Money
	Wallet         string
	Phone          string
	IdempotencyKey string
	Metadata       map[string]any
}

// ChargeResult is the sealed result of Initiate.
type ChargeResult interface{ isChargeResult() }

// ChargeSucceeded holds the reference for a settled charge.
type ChargeSucceeded struct {
	Ref         string
	Gateway     gateway.Gateway
	ExternalRef string
	SettledAt   time.Time
}

// ChargePending holds the reference for a pending charge.
type ChargePending struct {
	Ref         string
	Gateway     gateway.Gateway
	ExternalRef string
}

// ChargeFailed holds the failure reason.
type ChargeFailed struct{ Reason string }

func (ChargeSucceeded) isChargeResult() {}
func (ChargePending) isChargeResult()   {}
func (ChargeFailed) isChargeResult()    {}

// IntentStore is the persistence seam.
type IntentStore interface {
	CreateIntent(ctx context.Context, ref string, req ChargeRequest, gateway gateway.Gateway, amount money.Money) error
	FindByRef(ctx context.Context, ref string) (ChargePending, error)
	FindByIdempotencyKey(ctx context.Context, product, key string) (string, bool, error)
	UpdateStatus(ctx context.Context, ref string, status string) error
}

// Service orchestrates charges.
type Service struct {
	store  IntentStore
	router *gateway.ChargerRouter
}

// NewService builds a Service.
func NewService(store IntentStore, router *gateway.ChargerRouter, opts ...Option) *Service {
	s := &Service{store: store, router: router}
	for _, opt := range opts {
		opt(s)
	}
	return s
}

// Option configures Service.
type Option func(*Service)

// Initiate creates optd-{product}-{gateway}-{ulid}, persists, and calls the gateway.
func (s *Service) Initiate(ctx context.Context, req ChargeRequest) (ChargeResult, error) {
	ctx, span := observability.StartSpan(ctx, "charges.Initiate")
	defer span.End()

	if err := validate(req); err != nil {
		return nil, fmt.Errorf("%w: %w", ErrInvalidRequest, err)
	}
	wallet := req.Wallet
	if wallet == "" {
		wallet = req.Phone
	}
	// Idempotency: reuse existing ref if key seen.
	if req.IdempotencyKey != "" {
		if ref, found, err := s.store.FindByIdempotencyKey(ctx, req.Product, req.IdempotencyKey); err == nil && found {
			observability.SetEventField(ctx, "charge_ref", ref)
			return ChargePending{Ref: ref}, nil
		}
	}
	// Choose gateway order and generate reference with chosen gateway.
	var chosen gateway.Gateway
	var ref string
	for _, g := range s.router.Route(ctx) {
		ref = gateway.BuildReference(req.Product, g, gateway.NewULID())
		chosen = g
		break
	}
	if ref == "" {
		return nil, fmt.Errorf("charges: no gateway available: %w", gateway.ErrGatewayUnavailable)
	}
	if err := s.store.CreateIntent(ctx, ref, req, chosen, req.Amount); err != nil {
		return nil, fmt.Errorf("charges: create intent: %w", err)
	}
	adapter, ok := s.router.Adapter(chosen)
	if !ok {
		return ChargePending{Ref: ref, Gateway: chosen}, nil
	}
	resp, err := adapter.Initiate(ctx, gateway.InitiateRequest{
		Reference:      ref,
		Amount:         req.Amount,
		Wallet:         wallet,
		Product:        req.Product,
		IdempotencyKey: req.IdempotencyKey,
	})
	if err != nil {
		// Persist failure but keep intent for reconciliation.
		_ = s.store.UpdateStatus(ctx, ref, "failed")
		if errors.Is(err, gateway.ErrNotConfigured) {
			observability.LoggerFromContext(ctx).InfoContext(ctx, "gateway not configured", "gateway", chosen)
			return ChargePending{Ref: ref, Gateway: chosen, ExternalRef: ref}, nil
		}
		// Not-configured is not a gateway health signal; all other failures are.
		s.router.RecordResult(ctx, chosen, false)
		return ChargeFailed{Reason: err.Error()}, nil
	}
	s.router.RecordResult(ctx, chosen, true)
	observability.SetEventField(ctx, "charge_ref", ref)
	return ChargePending{Ref: ref, Gateway: chosen, ExternalRef: resp.ExternalRef}, nil
}

// Status calls the gateway Verify for authoritative truth.
func (s *Service) Status(ctx context.Context, ref string) (gateway.VerifyResult, error) {
	ctx, span := observability.StartSpan(ctx, "charges.Status")
	defer span.End()
	_, gw, _, ok := gateway.ParseReference(ref)
	if !ok {
		return gateway.VerifyResult{}, fmt.Errorf("%w: malformed reference %q", ErrInvalidRequest, ref)
	}
	adapter, exists := s.router.Adapter(gw)
	if !exists {
		return gateway.VerifyResult{}, fmt.Errorf("charges: no adapter for gateway %s", gw)
	}
	result, err := adapter.Verify(ctx, ref)
	if err != nil {
		return gateway.VerifyResult{}, fmt.Errorf("charges: verify: %w", err)
	}
	return result, nil
}

func validate(req ChargeRequest) error {
	if req.Product == "" {
		return errors.New("product is required")
	}
	if req.Amount.IsNegative() || req.Amount.IsZero() {
		return errors.New("amount must be positive")
	}
	if req.Wallet == "" && req.Phone == "" {
		return errors.New("wallet or phone is required")
	}
	return nil
}
