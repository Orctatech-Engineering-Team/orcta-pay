package webhooks

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/Orctatech-Engineering-Team/orcta-pay/internal/config"
	"github.com/Orctatech-Engineering-Team/orcta-pay/internal/gateway"
	"github.com/Orctatech-Engineering-Team/orcta-pay/internal/ledger"
	"github.com/Orctatech-Engineering-Team/orcta-pay/internal/money"
)

// fakeStore is an in-memory webhooks.Store.
type fakeStore struct {
	inbox    map[string][]byte
	intents  map[string]Intent
	outcomes map[string]string
	entries  map[string][]ledger.LedgerEntry
}

func newFakeStore() *fakeStore {
	return &fakeStore{
		inbox:    map[string][]byte{},
		intents:  map[string]Intent{},
		outcomes: map[string]string{},
		entries:  map[string][]ledger.LedgerEntry{},
	}
}

func (f *fakeStore) InsertWebhookInbox(_ context.Context, id, _ string, payload []byte) (bool, error) {
	if _, ok := f.inbox[id]; ok {
		return false, nil
	}
	f.inbox[id] = payload
	return true, nil
}

func (f *fakeStore) DeleteWebhookInbox(_ context.Context, id string) error {
	delete(f.inbox, id)
	return nil
}

func (f *fakeStore) FindIntent(_ context.Context, ref string) (Intent, error) {
	i, ok := f.intents[ref]
	if !ok {
		return Intent{}, ErrUnknownRef
	}
	return i, nil
}

func (f *fakeStore) ApplyChargeOutcome(_ context.Context, ref, status string, entries []ledger.LedgerEntry) error {
	f.outcomes[ref] = status
	if intent, ok := f.intents[ref]; ok {
		intent.Status = status
		f.intents[ref] = intent
	}
	f.entries[ref] = append(f.entries[ref], entries...)
	return nil
}

// fakeAdapter serves canned Verify results.
type fakeAdapter struct {
	verify func() (gateway.VerifyResult, error)
}

func (a *fakeAdapter) Initiate(context.Context, gateway.InitiateRequest) (gateway.InitiateResponse, error) {
	return gateway.InitiateResponse{}, nil
}
func (a *fakeAdapter) Verify(context.Context, string) (gateway.VerifyResult, error) {
	return a.verify()
}
func (a *fakeAdapter) Refund(context.Context, string, money.Money) error { return nil }
func (a *fakeAdapter) Payout(context.Context, string, money.Money, string) error {
	return nil
}

func newTestService(t *testing.T, store *fakeStore, adapter gateway.AggregatorClient) *Service {
	t.Helper()
	cfg := config.PaymentsConfig{Primary: "paystack"}
	router := gateway.NewChargerRouter(cfg, nil, map[gateway.Gateway]gateway.AggregatorClient{
		gateway.GatewayPaystack: adapter,
	})
	return NewService(store, router)
}

const testRef = "optd-test-paystack-01ARZ3NDEKTSV4RRFFQ69G5FAV"

func TestProcessSucceeded(t *testing.T) {
	store := newFakeStore()
	store.intents[testRef] = Intent{Ref: testRef, Gateway: gateway.GatewayPaystack, Status: "pending", Product: "test"}
	svc := newTestService(t, store, &fakeAdapter{verify: func() (gateway.VerifyResult, error) {
		return gateway.VerifyResult{
			Reference:  testRef,
			Status:     "succeeded",
			Amount:     money.New(1800, money.GHS),
			VerifiedAt: time.Now().UTC(),
		}, nil
	}})

	payload := []byte(`{"event":"charge.success","data":{"id":7,"reference":"` + testRef + `"}}`)
	if err := svc.Process(context.Background(), gateway.GatewayPaystack, payload); err != nil {
		t.Fatalf("Process: %v", err)
	}
	if store.outcomes[testRef] != "succeeded" {
		t.Fatalf("status = %q, want succeeded", store.outcomes[testRef])
	}
	entries := store.entries[testRef]
	if len(entries) != 1 {
		t.Fatalf("entries = %d, want 1 collection entry", len(entries))
	}
	if entries[0].Kind != ledger.KindCollection || entries[0].Amount.MinorUnits() != 1800 {
		t.Fatalf("entry = %+v", entries[0])
	}

	// Duplicate event: ErrDuplicate, no double ledger write.
	err := svc.Process(context.Background(), gateway.GatewayPaystack, payload)
	if !errors.Is(err, ErrDuplicate) {
		t.Fatalf("second Process err = %v, want ErrDuplicate", err)
	}
	if len(store.entries[testRef]) != 1 {
		t.Fatal("duplicate wrote a second ledger entry")
	}
}

func TestProcessAlreadyTerminal(t *testing.T) {
	store := newFakeStore()
	store.intents[testRef] = Intent{Ref: testRef, Gateway: gateway.GatewayPaystack, Status: "succeeded", Product: "test"}
	svc := newTestService(t, store, &fakeAdapter{verify: func() (gateway.VerifyResult, error) {
		t.Fatal("verify should not be called for terminal intents")
		return gateway.VerifyResult{}, nil
	}})
	payload := []byte(`{"event":"charge.success","data":{"id":8,"reference":"` + testRef + `"}}`)
	if err := svc.Process(context.Background(), gateway.GatewayPaystack, payload); err != nil {
		t.Fatalf("Process: %v", err)
	}
}

func TestProcessUnknownRefRollsBackInbox(t *testing.T) {
	store := newFakeStore()
	svc := newTestService(t, store, &fakeAdapter{verify: func() (gateway.VerifyResult, error) {
		return gateway.VerifyResult{}, nil
	}})
	payload := []byte(`{"event":"charge.success","data":{"id":9,"reference":"optd-missing-01ARZ3NDEKTSV4RRFFQ69G5FAV"}}`)
	err := svc.Process(context.Background(), gateway.GatewayPaystack, payload)
	if !errors.Is(err, ErrUnknownRef) {
		t.Fatalf("err = %v, want ErrUnknownRef", err)
	}
	if len(store.inbox) != 0 {
		t.Fatal("inbox row should be rolled back for unknown ref")
	}
}

func TestProcessVerifyFailureRollsBackInboxForRetry(t *testing.T) {
	store := newFakeStore()
	store.intents[testRef] = Intent{Ref: testRef, Gateway: gateway.GatewayPaystack, Status: "pending", Product: "test"}
	calls := 0
	svc := newTestService(t, store, &fakeAdapter{verify: func() (gateway.VerifyResult, error) {
		calls++
		if calls == 1 {
			return gateway.VerifyResult{}, errors.New("gateway timeout")
		}
		return gateway.VerifyResult{Reference: testRef, Status: "succeeded", Amount: money.New(100, money.GHS), VerifiedAt: time.Now().UTC()}, nil
	}})
	payload := []byte(`{"event":"charge.success","data":{"id":10,"reference":"` + testRef + `"}}`)

	if err := svc.Process(context.Background(), gateway.GatewayPaystack, payload); err == nil {
		t.Fatal("want verify error")
	}
	if len(store.inbox) != 0 {
		t.Fatal("inbox row should be dropped so the retry reprocesses")
	}
	if err := svc.Process(context.Background(), gateway.GatewayPaystack, payload); err != nil {
		t.Fatalf("retry Process: %v", err)
	}
	if store.outcomes[testRef] != "succeeded" {
		t.Fatal("retry should succeed after transient failure")
	}
}

func TestProcessFailedCharge(t *testing.T) {
	store := newFakeStore()
	store.intents[testRef] = Intent{Ref: testRef, Gateway: gateway.GatewayPaystack, Status: "pending", Product: "test"}
	svc := newTestService(t, store, &fakeAdapter{verify: func() (gateway.VerifyResult, error) {
		return gateway.VerifyResult{Reference: testRef, Status: "failed", VerifiedAt: time.Now().UTC()}, nil
	}})
	payload := []byte(`{"event":"charge.failed","data":{"id":11,"reference":"` + testRef + `"}}`)
	if err := svc.Process(context.Background(), gateway.GatewayPaystack, payload); err != nil {
		t.Fatalf("Process: %v", err)
	}
	if store.outcomes[testRef] != "failed" {
		t.Fatalf("status = %q", store.outcomes[testRef])
	}
	if len(store.entries[testRef]) != 0 {
		t.Fatal("failed charge must not write ledger entries")
	}
}

func TestProcessPendingLeavesIntent(t *testing.T) {
	store := newFakeStore()
	store.intents[testRef] = Intent{Ref: testRef, Gateway: gateway.GatewayPaystack, Status: "pending", Product: "test"}
	svc := newTestService(t, store, &fakeAdapter{verify: func() (gateway.VerifyResult, error) {
		return gateway.VerifyResult{Reference: testRef, Status: "pending", VerifiedAt: time.Now().UTC()}, nil
	}})
	payload := []byte(`{"event":"charge.pending","data":{"id":12,"reference":"` + testRef + `"}}`)
	if err := svc.Process(context.Background(), gateway.GatewayPaystack, payload); err != nil {
		t.Fatalf("Process: %v", err)
	}
	if store.outcomes[testRef] != "" {
		t.Fatal("pending verify should not change intent status")
	}
}
