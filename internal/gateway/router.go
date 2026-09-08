package gateway

import (
	"context"
	"errors"
	"fmt"

	"github.com/Orctatech-Engineering-Team/orcta-pay/internal/config"
)

// HealthStore reports gateway health for ranking. Implemented by storage/valkey.
type HealthStore interface {
	SuccessRate(ctx context.Context, gateway Gateway) (float64, error)
	IsCircuitOpen(ctx context.Context, gateway Gateway) (bool, error)
}

// ChargerRouter selects the gateway to try first, ranking by Valkey metrics.
type ChargerRouter struct {
	primary  Gateway
	adapters map[Gateway]AggregatorClient
	health   HealthStore
}

// NewChargerRouter builds the router. Primary is config.PAYMENTS_PRIMARY.
func NewChargerRouter(cfg config.PaymentsConfig, health HealthStore, adapters map[Gateway]AggregatorClient) *ChargerRouter {
	primary := Gateway(cfg.Primary)
	if !primary.Valid() {
		primary = GatewayPaystack
	}
	return &ChargerRouter{primary: primary, adapters: adapters, health: health}
}

// Route returns the ordered list of gateways to try, excluding circuit-open ones.
func (r *ChargerRouter) Route(ctx context.Context) []Gateway {
	order := []Gateway{r.primary}
	for _, g := range []Gateway{GatewayHubtel, GatewayPaystack, GatewayMoolre} {
		if g != r.primary {
			order = append(order, g)
		}
	}
	if r.health == nil {
		return order
	}
	var out []Gateway
	for _, g := range order {
		open, err := r.health.IsCircuitOpen(ctx, g)
		if err != nil || open {
			continue
		}
		out = append(out, g)
	}
	if len(out) == 0 {
		return order
	}
	return out
}

// InitiateWithFallback tries gateways in ranked order until one succeeds.
// It rebuilds the reference per gateway when the incoming reference is parseable
// so the optd-{product}-{gateway}-{ulid} segment matches the attempted gateway,
// and it records health outcomes via RecordResult (skipping ErrNotConfigured).
func (r *ChargerRouter) InitiateWithFallback(ctx context.Context, req InitiateRequest) (InitiateResponse, Gateway, error) {
	var lastErr error
	var product, ulid string
	canRebuild := false
	if prod, _, u, ok := ParseReference(req.Reference); ok {
		product = prod
		ulid = u
		canRebuild = true
	}
	for _, g := range r.Route(ctx) {
		adapter, ok := r.adapters[g]
		if !ok {
			continue
		}
		attemptReq := req
		if canRebuild {
			attemptReq.Reference = BuildReference(product, g, ulid)
		}
		resp, err := adapter.Initiate(ctx, attemptReq)
		if err == nil {
			r.RecordResult(ctx, g, true)
			return resp, g, nil
		}
		if errors.Is(err, ErrNotConfigured) {
			lastErr = err
			continue
		}
		r.RecordResult(ctx, g, false)
		lastErr = fmt.Errorf("gateway %s: %w", g, err)
	}
	if lastErr == nil {
		lastErr = ErrGatewayUnavailable
	}
	return InitiateResponse{}, "", lastErr
}

// Adapter returns the adapter for gateway.
func (r *ChargerRouter) Adapter(g Gateway) (AggregatorClient, bool) {
	a, ok := r.adapters[g]
	return a, ok
}

// RecordResult forwards an outcome to the health store when one is wired.
// Never fatal: health tracking must not break charge processing.
func (r *ChargerRouter) RecordResult(ctx context.Context, g Gateway, success bool) {
	if r.health == nil {
		return
	}
	if rec, ok := r.health.(ResultRecorder); ok {
		_ = rec.RecordResult(ctx, g, success)
	}
}
