package gateway

import (
	"context"
	"errors"
	"fmt"

	"github.com/orctatech/orcta-pay/internal/config"
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
func (r *ChargerRouter) InitiateWithFallback(ctx context.Context, req InitiateRequest) (InitiateResponse, Gateway, error) {
	var lastErr error
	for _, g := range r.Route(ctx) {
		adapter, ok := r.adapters[g]
		if !ok {
			continue
		}
		resp, err := adapter.Initiate(ctx, req)
		if err == nil {
			return resp, g, nil
		}
		if errors.Is(err, ErrNotConfigured) {
			lastErr = err
			continue
		}
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
