package valkey

import (
	"context"
	"fmt"
	"strconv"
	"time"

	"github.com/valkey-io/valkey-go"

	"github.com/orctatech/orcta-pay/internal/gateway"
)

// HealthStore tracks rolling success rates and circuit breaker state in Valkey.
//
// Key layout (per gateway, keyPrefix-scoped):
//   - {p}:health:{gw}:s        successes in the rolling window
//   - {p}:health:{gw}:t        attempts in the rolling window
//   - {p}:health:{gw}:f        consecutive failures (reset on success)
//   - {p}:health:{gw}:circuit  exists ⇔ circuit open; TTL = open duration
//
// The circuit opens after failureThreshold consecutive failures and
// half-closes automatically when the open-duration TTL expires; the next
// attempt then either resets (success) or re-opens (failure).
type HealthStore struct {
	client valkey.Client
	prefix string
}

// Breaker tuning knobs.
const (
	// healthWindowTTL expires the rolling counters so the window slides.
	healthWindowTTL = 10 * time.Minute
	// failureThreshold consecutive failures before the circuit opens.
	failureThreshold = 5
	// circuitOpenTTL is how long the circuit stays open before half-closing.
	circuitOpenTTL = 30 * time.Second
)

// NewHealthStore builds a HealthStore.
func NewHealthStore(client valkey.Client) *HealthStore {
	return &HealthStore{client: client, prefix: "orctapay"}
}

var _ gateway.HealthStore = (*HealthStore)(nil)

func (s *HealthStore) key(gw gateway.Gateway, suffix string) string {
	return fmt.Sprintf("%s:health:%s:%s", s.prefix, gw, suffix)
}

func (s *HealthStore) circuitKey(gw gateway.Gateway) string {
	return fmt.Sprintf("%s:health:%s:circuit", s.prefix, gw)
}

// SuccessRate returns the rolling success rate for gateway. Unknown gateways
// (no attempts recorded) report 1.0 so they are not deprioritized.
func (s *HealthStore) SuccessRate(ctx context.Context, gw gateway.Gateway) (float64, error) {
	if s.client == nil {
		return 1.0, nil
	}
	resp := s.client.Do(ctx, s.client.B().Mget().Key(s.key(gw, "s"), s.key(gw, "t")).Build())
	arr, err := resp.ToArray()
	if err != nil {
		return 1.0, fmt.Errorf("valkey: mget health: %w", err)
	}
	var vals [2]float64
	for i, m := range arr {
		v, err := m.ToString()
		if err != nil {
			if valkey.IsValkeyNil(err) {
				continue // missing counter
			}
			return 1.0, fmt.Errorf("valkey: mget health: %w", err)
		}
		vals[i], _ = strconv.ParseFloat(v, 64)
	}
	if vals[1] <= 0 {
		return 1.0, nil
	}
	return vals[0] / vals[1], nil
}

// IsCircuitOpen reports whether the circuit is open.
func (s *HealthStore) IsCircuitOpen(ctx context.Context, gw gateway.Gateway) (bool, error) {
	if s.client == nil {
		return false, nil
	}
	resp := s.client.Do(ctx, s.client.B().Exists().Key(s.circuitKey(gw)).Build())
	n, err := resp.AsInt64()
	if err != nil {
		return false, fmt.Errorf("valkey: exists circuit: %w", err)
	}
	return n > 0, nil
}

// RecordResult records an outcome: attempts/successes for the rolling window
// and consecutive-failure counting for the breaker.
func (s *HealthStore) RecordResult(ctx context.Context, gw gateway.Gateway, success bool) error {
	if s.client == nil {
		return nil
	}
	cmds := []valkey.Completed{
		s.client.B().Incr().Key(s.key(gw, "t")).Build(),
		s.client.B().Expire().Key(s.key(gw, "t")).Seconds(int64(healthWindowTTL.Seconds())).Build(),
	}
	if success {
		cmds = append(cmds,
			s.client.B().Incr().Key(s.key(gw, "s")).Build(),
			s.client.B().Expire().Key(s.key(gw, "s")).Seconds(int64(healthWindowTTL.Seconds())).Build(),
			s.client.B().Del().Key(s.key(gw, "f")).Build(),
		)
	} else {
		cmds = append(cmds, s.client.B().Incr().Key(s.key(gw, "f")).Build())
	}
	// RecordResult: batched; every response checked.
	resp := s.client.DoMulti(ctx, cmds...)
	for _, r := range resp {
		if err := r.Error(); err != nil {
			return fmt.Errorf("valkey: record result: %w", err)
		}
	}
	if !success {
		return s.maybeOpenCircuit(ctx, gw)
	}
	return nil
}

// maybeOpenCircuit opens the breaker when consecutive failures reach the
// threshold. The open key's TTL doubles as the half-open cooldown.
func (s *HealthStore) maybeOpenCircuit(ctx context.Context, gw gateway.Gateway) error {
	resp := s.client.Do(ctx, s.client.B().Get().Key(s.key(gw, "f")).Build())
	v, err := resp.ToString()
	if err != nil {
		if valkey.IsValkeyNil(err) {
			return nil
		}
		return fmt.Errorf("valkey: get failures: %w", err)
	}
	fails, _ := strconv.Atoi(v)
	if fails < failureThreshold {
		return nil
	}
	if err := s.client.Do(ctx, s.client.B().Set().Key(s.circuitKey(gw)).Value("open").ExSeconds(int64(circuitOpenTTL.Seconds())).Build()).Error(); err != nil {
		return fmt.Errorf("valkey: open circuit: %w", err)
	}
	return nil
}
