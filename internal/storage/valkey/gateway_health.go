package valkey

import (
	"context"
	"fmt"
	"math"
	"strconv"
	"strings"
	"sync/atomic"
	"time"

	"github.com/valkey-io/valkey-go"

	"github.com/Orctatech-Engineering-Team/orcta-pay/internal/gateway"
)

var latencySeq atomic.Uint64

// HealthStore tracks rolling success rates, circuit breaker state, and latency histogram in Valkey.
//
// Key layout (per gateway, keyPrefix-scoped):
//   - {p}:health:{gw}:s        successes in the rolling window
//   - {p}:health:{gw}:t        attempts in the rolling window
//   - {p}:health:{gw}:f        consecutive failures (reset on success)
//   - {p}:health:{gw}:circuit  exists ⇔ circuit open; TTL = open duration
//   - {p}:health:{gw}:lat      sorted set of latency samples (score = latency_ms, member = "<unix_nano>:<seq>:<ms>")
//
// The circuit opens after failureThreshold consecutive failures and
// half-closes automatically when the open-duration TTL expires; the next
// attempt then either resets (success) or re-opens (failure).
// Latency is sampled via a Valkey sorted set (score = ms); p95 is the
// ceil(0.95*n)-th element. Stale samples older than healthWindowTTL are
// pruned lazily on read (and opportunistically on write).
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
var _ gateway.LatencyRecorder = (*HealthStore)(nil)

func (s *HealthStore) key(gw gateway.Gateway, suffix string) string {
	return fmt.Sprintf("%s:health:%s:%s", s.prefix, gw, suffix)
}

func (s *HealthStore) circuitKey(gw gateway.Gateway) string {
	return fmt.Sprintf("%s:health:%s:circuit", s.prefix, gw)
}

func (s *HealthStore) latencyKey(gw gateway.Gateway) string {
	return fmt.Sprintf("%s:health:%s:lat", s.prefix, gw)
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

// RecordLatency records a latency sample for p95 calculation.
// Latency is stored as a sorted-set score (ms) with a unique member that
// encodes the sample timestamp for windowed pruning. The set is expired
// after healthWindowTTL so idle gateways naturally reset.
func (s *HealthStore) RecordLatency(ctx context.Context, gw gateway.Gateway, latency time.Duration) error {
	if s.client == nil {
		return nil
	}
	ms := latency.Milliseconds()
	if ms < 0 {
		ms = 0
	}
	if ms > 60000 {
		ms = 60000
	}
	seq := latencySeq.Add(1)
	member := fmt.Sprintf("%d:%d:%d", time.Now().UnixNano(), seq, ms)
	key := s.latencyKey(gw)
	cmds := []valkey.Completed{
		s.client.B().Zadd().Key(key).ScoreMember().ScoreMember(float64(ms), member).Build(),
		s.client.B().Expire().Key(key).Seconds(int64(healthWindowTTL.Seconds())).Build(),
	}
	resp := s.client.DoMulti(ctx, cmds...)
	for _, r := range resp {
		if err := r.Error(); err != nil {
			return fmt.Errorf("valkey: record latency: %w", err)
		}
	}
	return nil
}

// P95Latency returns the p95 latency in milliseconds for the gateway's
// recent window. Stale samples older than healthWindowTTL are pruned
// lazily. Returns 0 when no samples exist.
func (s *HealthStore) P95Latency(ctx context.Context, gw gateway.Gateway) (int, error) {
	if s.client == nil {
		return 0, nil
	}
	key := s.latencyKey(gw)
	resp := s.client.Do(ctx, s.client.B().Zrange().Key(key).Min("0").Max("-1").Withscores().Build())
	scores, err := resp.AsZScores()
	if err != nil {
		if valkey.IsValkeyNil(err) {
			return 0, nil
		}
		return 0, fmt.Errorf("valkey: zrange lat: %w", err)
	}
	if len(scores) == 0 {
		return 0, nil
	}
	now := time.Now()
	cutoff := now.Add(-healthWindowTTL).UnixNano()
	var latencies []int
	var stale []string
	for _, zs := range scores {
		member := zs.Member
		score := zs.Score
		// Prune check: member format "<nano>:<seq>:<ms>"
		tsStr := member
		if idx := strings.Index(member, ":"); idx != -1 {
			tsStr = member[:idx]
		}
		if ts, err := strconv.ParseInt(tsStr, 10, 64); err == nil {
			if ts < cutoff {
				stale = append(stale, member)
				continue
			}
		}
		latencies = append(latencies, int(math.Round(score)))
	}
	if len(stale) > 0 {
		_ = s.client.Do(ctx, s.client.B().Zrem().Key(key).Member(stale...).Build()).Error()
	}
	if len(latencies) == 0 {
		return 0, nil
	}
	// ZRANGE already returns sorted ascending by score, but after filtering
	// stale entries the order is preserved. Nonetheless ensure sorted for
	// correctness when stale filtering happened out-of-order.
	// Since latencies came from sorted scores they remain sorted; no extra sort needed.
	n := len(latencies)
	idx := int(math.Ceil(0.95*float64(n))) - 1
	if idx < 0 {
		idx = 0
	}
	if idx >= n {
		idx = n - 1
	}
	return latencies[idx], nil
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
