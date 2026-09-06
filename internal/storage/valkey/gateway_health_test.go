package valkey

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/valkey-io/valkey-go"

	"github.com/Orctatech-Engineering-Team/orcta-pay/internal/gateway"
)

// newTestClient skips unless TEST_VALKEY_ADDR points at a reachable Valkey.
// Local: task dev:up starts one.
func newTestClient(t *testing.T) valkey.Client {
	t.Helper()
	addr := os.Getenv("TEST_VALKEY_ADDR")
	if addr == "" {
		t.Skip("TEST_VALKEY_ADDR not set; skipping Valkey integration test")
	}
	client, err := valkey.NewClient(valkey.ClientOption{InitAddress: []string{addr}})
	if err != nil {
		t.Fatalf("connect valkey: %v", err)
	}
	t.Cleanup(client.Close)
	return client
}

func TestHealthStoreRollingWindow(t *testing.T) {
	client := newTestClient(t)
	ctx := context.Background()
	s := NewHealthStore(client)
	gw := gateway.Gateway("test-gw-" + time.Now().UTC().Format("150405.000000000"))
	t.Cleanup(func() {
		_ = client.Do(context.Background(), client.B().Del().Key(
			s.key(gw, "s"), s.key(gw, "t"), s.key(gw, "f"), s.circuitKey(gw),
		).Build()).Error()
	})

	// No attempts yet: perfect score, circuit closed.
	rate, err := s.SuccessRate(ctx, gw)
	if err != nil || rate != 1.0 {
		t.Fatalf("initial rate = %v, %v; want 1.0", rate, err)
	}
	open, err := s.IsCircuitOpen(ctx, gw)
	if err != nil || open {
		t.Fatalf("initial circuit = %v, %v; want closed", open, err)
	}

	// 3 successes → rate 1.0.
	for i := 0; i < 3; i++ {
		if err := s.RecordResult(ctx, gw, true); err != nil {
			t.Fatalf("RecordResult success: %v", err)
		}
	}
	rate, err = s.SuccessRate(ctx, gw)
	if err != nil || rate != 1.0 {
		t.Fatalf("rate after successes = %v, %v; want 1.0", rate, err)
	}

	// 1 failure → rate 0.75.
	if err := s.RecordResult(ctx, gw, false); err != nil {
		t.Fatalf("RecordResult failure: %v", err)
	}
	rate, err = s.SuccessRate(ctx, gw)
	if err != nil || rate != 0.75 {
		t.Fatalf("rate after failure = %v, %v; want 0.75", rate, err)
	}

	// Success resets consecutive failures; the breaker must not open until
	// failureThreshold failures in a row.
	if err := s.RecordResult(ctx, gw, true); err != nil {
		t.Fatalf("RecordResult success: %v", err)
	}
	for i := 0; i < failureThreshold-1; i++ {
		if err := s.RecordResult(ctx, gw, false); err != nil {
			t.Fatalf("RecordResult failure: %v", err)
		}
		if open, _ := s.IsCircuitOpen(ctx, gw); open {
			t.Fatal("circuit opened below threshold")
		}
	}
	if err := s.RecordResult(ctx, gw, false); err != nil {
		t.Fatalf("RecordResult failure: %v", err)
	}
	open, err = s.IsCircuitOpen(ctx, gw)
	if err != nil || !open {
		t.Fatalf("circuit = %v, %v; want open after %d consecutive failures", open, err, failureThreshold)
	}
}

func TestLockerAcquireRelease(t *testing.T) {
	client := newTestClient(t)
	ctx := context.Background()
	l := NewLocker(client)
	key := "orctapay:test-lock-" + time.Now().UTC().Format("150405.000000000")
	t.Cleanup(func() { _ = l.Release(ctx, key) })

	ok, err := l.TryAcquire(ctx, key, 5*time.Second)
	if err != nil || !ok {
		t.Fatalf("first acquire = %v, %v; want true", ok, err)
	}
	ok, err = l.TryAcquire(ctx, key, 5*time.Second)
	if err != nil || ok {
		t.Fatalf("second acquire = %v, %v; want false", ok, err)
	}
	if err := l.Release(ctx, key); err != nil {
		t.Fatalf("Release: %v", err)
	}
	ok, err = l.TryAcquire(ctx, key, 5*time.Second)
	if err != nil || !ok {
		t.Fatalf("acquire after release = %v, %v; want true", ok, err)
	}
}
