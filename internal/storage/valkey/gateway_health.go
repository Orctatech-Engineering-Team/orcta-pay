package valkey

import (
	"context"
	"time"

	"github.com/valkey-io/valkey-go"

	"github.com/orctatech/orcta-pay/internal/gateway"
)

// HealthStore tracks rolling success rates and circuit breaker state in Valkey.
type HealthStore struct {
	client valkey.Client
}

// NewHealthStore builds a HealthStore.
func NewHealthStore(client valkey.Client) *HealthStore { return &HealthStore{client: client} }

var _ gateway.HealthStore = (*HealthStore)(nil)

// SuccessRate returns the rolling success rate for gateway.
func (s *HealthStore) SuccessRate(ctx context.Context, gw gateway.Gateway) (float64, error) {
	if s.client == nil {
		return 1.0, nil
	}
	// Stub: real implementation reads rolling window from Valkey.
	return 1.0, nil
}

// IsCircuitOpen reports whether the circuit is open.
func (s *HealthStore) IsCircuitOpen(ctx context.Context, gw gateway.Gateway) (bool, error) {
	if s.client == nil {
		return false, nil
	}
	return false, nil
}

// RecordResult records an outcome for ranking.
func (s *HealthStore) RecordResult(ctx context.Context, gw gateway.Gateway, success bool) error {
	if s.client == nil {
		return nil
	}
	// TTL window key omitted in stub.
	_ = time.Now
	return nil
}
