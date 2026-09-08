// Package charges hides collection lifecycle and reference generation.
package charges

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/Orctatech-Engineering-Team/orcta-pay/internal/gateway"
	"github.com/Orctatech-Engineering-Team/orcta-pay/internal/money"
	"github.com/Orctatech-Engineering-Team/orcta-pay/internal/observability"
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
	Ref              string
	Gateway          gateway.Gateway
	ExternalRef      string
	AuthorizationURL string
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
// It preserves the idempotency guard before routing and uses InitiateWithFallback
// for ranked fallback, recording health and persisting the actual chosen gateway.
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
	// Idempotency: reuse existing ref if key seen. Must stay before any routing or side effects.
	if req.IdempotencyKey != "" {
		if ref, found, err := s.store.FindByIdempotencyKey(ctx, req.Product, req.IdempotencyKey); err == nil && found {
			observability.SetEventField(ctx, "charge_ref", ref)
			return ChargePending{Ref: ref}, nil
		}
	}
	ordered := s.router.Route(ctx)
	if len(ordered) == 0 {
		return nil, fmt.Errorf("charges: no gateway available: %w", gateway.ErrGatewayUnavailable)
	}
	ulid := gateway.NewULID()
	baseRef := gateway.BuildReference(req.Product, ordered[0], ulid)
	baseReq := gateway.InitiateRequest{
		Reference:      baseRef,
		Amount:         req.Amount,
		Wallet:         wallet,
		Product:        req.Product,
		IdempotencyKey: req.IdempotencyKey,
	}
	resp, chosen, err := s.router.InitiateWithFallback(ctx, baseReq)
	if err != nil {
		if errors.Is(err, gateway.ErrNotConfigured) {
			chosen = ordered[0]
			// Persist intent so it remains findable for reconciliation.
			_ = s.store.CreateIntent(ctx, baseRef, req, chosen, req.Amount)
			observability.LoggerFromContext(ctx).InfoContext(ctx, "gateway not configured", "gateway", chosen)
			observability.SetEventField(ctx, "charge_ref", baseRef)
			return ChargePending{Ref: baseRef, Gateway: chosen, ExternalRef: baseRef}, nil
		}
		return ChargeFailed{Reason: err.Error()}, nil
	}
	// Build the actual reference for the chosen gateway reusing the same ULID.
	actualRef := baseRef
	if prod, g, parsedULID, ok := gateway.ParseReference(baseRef); ok && g != chosen {
		actualRef = gateway.BuildReference(prod, chosen, parsedULID)
	}
	if err := s.store.CreateIntent(ctx, actualRef, req, chosen, req.Amount); err != nil {
		return nil, fmt.Errorf("charges: create intent: %w", err)
	}
	observability.SetEventField(ctx, "charge_ref", actualRef)
	return ChargePending{Ref: actualRef, Gateway: chosen, ExternalRef: resp.ExternalRef, AuthorizationURL: resp.AuthorizationURL}, nil
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
