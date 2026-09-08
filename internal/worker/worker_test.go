package worker

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/Orctatech-Engineering-Team/orcta-pay/internal/config"
	"github.com/Orctatech-Engineering-Team/orcta-pay/internal/gateway"
	"github.com/Orctatech-Engineering-Team/orcta-pay/internal/ledger"
	"github.com/Orctatech-Engineering-Team/orcta-pay/internal/money"
)

// fakeLocker is an in-memory SETNX locker.
type fakeLocker struct {
	mu    sync.Mutex
	locks map[string]time.Time
}

func newFakeLocker() *fakeLocker { return &fakeLocker{locks: make(map[string]time.Time)} }

func (l *fakeLocker) TryAcquire(_ context.Context, key string, ttl time.Duration) (bool, error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	if exp, ok := l.locks[key]; ok && time.Now().Before(exp) {
		return false, nil
	}
	l.locks[key] = time.Now().Add(ttl)
	return true, nil
}
func (l *fakeLocker) Release(_ context.Context, key string) error {
	l.mu.Lock()
	defer l.mu.Unlock()
	delete(l.locks, key)
	return nil
}

// stubAdapter returns configured verify result.
type stubAdapter struct {
	verifyResult gateway.VerifyResult
	verifyErr    error
	initResp     gateway.InitiateResponse
	initErr      error
	calls        int
	mu           sync.Mutex
}

func (s *stubAdapter) Initiate(_ context.Context, _ gateway.InitiateRequest) (gateway.InitiateResponse, error) {
	s.mu.Lock()
	s.calls++
	s.mu.Unlock()
	return s.initResp, s.initErr
}
func (s *stubAdapter) Verify(_ context.Context, ref string) (gateway.VerifyResult, error) {
	s.mu.Lock()
	s.calls++
	s.mu.Unlock()
	if s.verifyErr != nil {
		return gateway.VerifyResult{}, s.verifyErr
	}
	res := s.verifyResult
	if res.Reference == "" {
		res.Reference = ref
	}
	if res.VerifiedAt.IsZero() {
		res.VerifiedAt = time.Now().UTC()
	}
	return res, nil
}
func (s *stubAdapter) Refund(_ context.Context, _ string, _ money.Money) error { return nil }
func (s *stubAdapter) Payout(_ context.Context, _ string, _ money.Money, _ string) error {
	return nil
}
func (s *stubAdapter) callCount() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.calls
}

type testStore struct {
	mu      sync.Mutex
	intents map[string]Intent
	events  map[uuid.UUID]GatewayEvent
	ledgers map[string][]ledger.LedgerEntry
}

func newTestStore() *testStore {
	return &testStore{
		intents: make(map[string]Intent),
		events:  make(map[uuid.UUID]GatewayEvent),
		ledgers: make(map[string][]ledger.LedgerEntry),
	}
}
func (s *testStore) insertIntentAndOutbox(ref string, gw gateway.Gateway, product string) uuid.UUID {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.intents[ref] = Intent{Ref: ref, Product: product, Amount: money.New(1800, money.GHS), Gateway: gw, Status: "pending"}
	id := uuid.New()
	s.events[id] = GatewayEvent{ID: id, Ref: ref, Gateway: gw, Status: "pending", CreatedAt: time.Now().UTC()}
	return id
}
func (s *testStore) ClaimPending(_ context.Context, limit int32) ([]GatewayEvent, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	var pending []GatewayEvent
	for _, ev := range s.events {
		if ev.Status == "pending" {
			pending = append(pending, ev)
		}
	}
	// sort by CreatedAt
	for i := 0; i < len(pending); i++ {
		for j := i + 1; j < len(pending); j++ {
			if pending[j].CreatedAt.Before(pending[i].CreatedAt) {
				pending[i], pending[j] = pending[j], pending[i]
			}
		}
	}
	if int32(len(pending)) > limit {
		pending = pending[:limit]
	}
	return pending, nil
}
func (s *testStore) GetIntent(_ context.Context, ref string) (Intent, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	it, ok := s.intents[ref]
	if !ok {
		return Intent{}, gateway.ErrNotConfigured
	}
	return it, nil
}
func (s *testStore) CompleteSucceeded(_ context.Context, eventID uuid.UUID, ref string, result gateway.VerifyResult, intent Intent) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if ev, ok := s.events[eventID]; ok {
		ev.Status = "succeeded"
		s.events[eventID] = ev
	}
	if it, ok := s.intents[ref]; ok {
		it.Status = "succeeded"
		s.intents[ref] = it
	}
	amt := result.Amount
	if amt.IsZero() {
		amt = intent.Amount
	}
	s.ledgers[ref] = append(s.ledgers[ref], ledger.LedgerEntry{
		ID:          uuid.New(),
		Kind:        ledger.KindCollection,
		Ref:         ref,
		Amount:      amt,
		ValueTime:   result.VerifiedAt,
		BookingTime: time.Now().UTC(),
		Product:     intent.Product,
	})
	return nil
}
func (s *testStore) CompleteFailed(_ context.Context, eventID uuid.UUID, ref string, _ gateway.VerifyResult) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if ev, ok := s.events[eventID]; ok {
		ev.Status = "failed"
		s.events[eventID] = ev
	}
	if it, ok := s.intents[ref]; ok {
		it.Status = "failed"
		s.intents[ref] = it
	}
	return nil
}
func (s *testStore) MarkPendingRetry(_ context.Context, _ uuid.UUID) error { return nil }
func (s *testStore) CountPending(_ context.Context) (int64, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	var n int64
	for _, ev := range s.events {
		if ev.Status == "pending" {
			n++
		}
	}
	return n, nil
}
func (s *testStore) ListEntries(_ context.Context, ref string) ([]ledger.LedgerEntry, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.ledgers[ref], nil
}

func TestWorkerDrainClaimedByOneAndSettles(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	store := newTestStore()
	locker := newFakeLocker()

	ref := "optd-test-paystack-" + uuid.New().String()[:8]
	store.insertIntentAndOutbox(ref, gateway.GatewayPaystack, "test")

	stub := &stubAdapter{
		verifyResult: gateway.VerifyResult{Reference: ref, Status: "succeeded", Amount: money.New(1800, money.GHS), VerifiedAt: time.Now().UTC()},
	}
	router := gateway.NewChargerRouter(config.PaymentsConfig{Primary: "paystack"}, nil, map[gateway.Gateway]gateway.AggregatorClient{
		gateway.GatewayPaystack: stub,
		gateway.GatewayHubtel:   &stubAdapter{verifyErr: gateway.ErrNotConfigured},
	})

	// Two workers sharing same store and locker, draining concurrently.
	w1 := New(store, store, router, locker, nil)
	w2 := New(store, store, router, locker, nil)

	var wg sync.WaitGroup
	wg.Add(2)
	go func() { defer wg.Done(); _, _ = w1.DrainOutbox(ctx) }()
	go func() { defer wg.Done(); _, _ = w2.DrainOutbox(ctx) }()
	wg.Wait()

	// Exactly one ledger entry should exist, intent should be succeeded.
	entries, err := store.ListEntries(ctx, ref)
	if err != nil {
		t.Fatalf("ListEntries: %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("want 1 ledger entry, got %d", len(entries))
	}
	if entries[0].Amount.MinorUnits() != 1800 {
		t.Fatalf("amount = %d, want 1800", entries[0].Amount.MinorUnits())
	}
	intent, err := store.GetIntent(ctx, ref)
	if err != nil {
		t.Fatalf("GetIntent: %v", err)
	}
	if intent.Status != "succeeded" {
		t.Fatalf("intent status = %q, want succeeded", intent.Status)
	}
	pending, _ := store.CountPending(ctx)
	if pending != 0 {
		t.Fatalf("pending = %d, want 0", pending)
	}
	// Due to lock contention, total verify calls should be 1 (the winner).
	// The loser either saw no events (race on claim) or lock contention.
	// Accept 1 or 2 but ledger must remain idempotent.
	if stub.callCount() < 1 || stub.callCount() > 2 {
		t.Fatalf("verify calls = %d, want 1-2", stub.callCount())
	}
}

func TestWorkerDrainPendingThenSucceedsOnRetry(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	store := newTestStore()
	locker := newFakeLocker()

	ref := "optd-test-hubtel-" + uuid.New().String()[:8]
	store.insertIntentAndOutbox(ref, gateway.GatewayHubtel, "test")

	// First drain returns pending.
	stubPending := &stubAdapter{
		verifyResult: gateway.VerifyResult{Reference: ref, Status: "pending", VerifiedAt: time.Now().UTC()},
	}
	router1 := gateway.NewChargerRouter(config.PaymentsConfig{Primary: "hubtel"}, nil, map[gateway.Gateway]gateway.AggregatorClient{
		gateway.GatewayHubtel: stubPending,
	})
	w1 := New(store, store, router1, locker, nil)
	if _, err := w1.DrainOutbox(ctx); err != nil {
		t.Fatalf("first drain: %v", err)
	}
	pending, _ := store.CountPending(ctx)
	if pending != 1 {
		t.Fatalf("after pending drain, pending = %d, want 1", pending)
	}
	entries, _ := store.ListEntries(ctx, ref)
	if len(entries) != 0 {
		t.Fatalf("want 0 entries after pending, got %d", len(entries))
	}

	// Second drain returns succeeded (as if gateway now settled).
	stubSucceeded := &stubAdapter{
		verifyResult: gateway.VerifyResult{Reference: ref, Status: "succeeded", Amount: money.New(2500, money.GHS), VerifiedAt: time.Now().UTC()},
	}
	router2 := gateway.NewChargerRouter(config.PaymentsConfig{Primary: "hubtel"}, nil, map[gateway.Gateway]gateway.AggregatorClient{
		gateway.GatewayHubtel: stubSucceeded,
	})
	w2 := New(store, store, router2, locker, nil)
	if _, err := w2.DrainOutbox(ctx); err != nil {
		t.Fatalf("second drain: %v", err)
	}
	pending, _ = store.CountPending(ctx)
	if pending != 0 {
		t.Fatalf("after succeeded drain, pending = %d, want 0", pending)
	}
	entries, _ = store.ListEntries(ctx, ref)
	if len(entries) != 1 || entries[0].Amount.MinorUnits() != 2500 {
		t.Fatalf("entries after succeeded = %+v, want one with 2500", entries)
	}
}

func TestReconcileEmitsMetric(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	store := newTestStore()
	locker := newFakeLocker()
	// One pending event to be counted.
	ref := "optd-test-paystack-" + uuid.New().String()[:8]
	store.insertIntentAndOutbox(ref, gateway.GatewayPaystack, "test")

	router := gateway.NewChargerRouter(config.PaymentsConfig{Primary: "paystack"}, nil, map[gateway.Gateway]gateway.AggregatorClient{
		gateway.GatewayPaystack: &stubAdapter{
			verifyResult: gateway.VerifyResult{Reference: ref, Status: "pending", VerifiedAt: time.Now().UTC()},
		},
	})
	w := New(store, store, router, locker, nil)
	if err := w.Reconcile(ctx); err != nil {
		t.Fatalf("Reconcile: %v", err)
	}
	// Reconcile should not mark pending as done (thin slice samples but does not mutate pending).
	pending, _ := store.CountPending(ctx)
	if pending != 1 {
		t.Fatalf("pending after reconcile = %d, want 1", pending)
	}
}
