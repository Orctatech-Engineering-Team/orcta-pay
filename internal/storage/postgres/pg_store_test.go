package postgres

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/Orctatech-Engineering-Team/orcta-pay/internal/charges"
	"github.com/Orctatech-Engineering-Team/orcta-pay/internal/gateway"
	"github.com/Orctatech-Engineering-Team/orcta-pay/internal/ledger"
	"github.com/Orctatech-Engineering-Team/orcta-pay/internal/money"
	"github.com/Orctatech-Engineering-Team/orcta-pay/internal/payouts"
)

// newTestStore skips unless TEST_DATABASE_URL points at a migrated database.
// Local: task dev:up && task migrate:up, then
// TEST_DATABASE_URL=postgres://orcta:orcta@localhost:5432/orcta_pay?sslmode=disable go test ./internal/storage/postgres/
func newTestStore(t *testing.T) *PostgresStore {
	t.Helper()
	url := os.Getenv("TEST_DATABASE_URL")
	if url == "" {
		t.Skip("TEST_DATABASE_URL not set; skipping Postgres integration test")
	}
	pool, err := pgxpool.New(context.Background(), url)
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	t.Cleanup(pool.Close)
	return NewPostgresStore(pool)
}

func TestPostgresStoreCharges(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()
	ref := "optd-test-hubtel-" + uuid.New().String()[:8]
	req := charges.ChargeRequest{Product: "test", Wallet: "0241234567", IdempotencyKey: "key-" + ref}

	if err := s.CreateIntent(ctx, ref, req, gateway.GatewayPaystack, money.New(1800, money.GHS), "https://checkout.paystack.com/test"); err != nil {
		t.Fatalf("CreateIntent: %v", err)
	}
	got, err := s.FindByRef(ctx, ref)
	if err != nil {
		t.Fatalf("FindByRef: %v", err)
	}
	if got.Ref != ref || got.Gateway != gateway.GatewayPaystack {
		t.Fatalf("got = %+v", got)
	}
	if got.AuthorizationURL != "https://checkout.paystack.com/test" {
		t.Fatalf("AuthorizationURL = %q, want https://checkout.paystack.com/test", got.AuthorizationURL)
	}

	foundRef, ok, err := s.FindByIdempotencyKey(ctx, "test", "key-"+ref)
	if err != nil || !ok || foundRef != ref {
		t.Fatalf("FindByIdempotencyKey = %q, %v, %v", foundRef, ok, err)
	}
	_, ok, _ = s.FindByIdempotencyKey(ctx, "test", "missing")
	if ok {
		t.Fatal("want not found")
	}

	if err := s.UpdateStatus(ctx, ref, "succeeded"); err != nil {
		t.Fatalf("UpdateStatus: %v", err)
	}
}

func TestPostgresStoreLedgerAndSettlement(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()
	ref := "optd-test-hubtel-" + uuid.New().String()[:8]

	entry := ledger.LedgerEntry{
		ID:          uuid.New(),
		Kind:        ledger.KindCollection,
		Ref:         ref,
		Amount:      money.New(1800, money.GHS),
		ValueTime:   time.Now().UTC(),
		BookingTime: time.Now().UTC(),
		Product:     "test",
	}
	if err := s.AppendEntry(ctx, entry); err != nil {
		t.Fatalf("AppendEntry: %v", err)
	}
	entries, err := s.ListEntries(ctx, ref)
	if err != nil || len(entries) != 1 {
		t.Fatalf("ListEntries = %v, %v", entries, err)
	}
	if entries[0].SettlementTime != nil {
		t.Fatal("settlement should start unset")
	}

	if err := s.StampSettlement(ctx, ref, time.Now().UTC()); err != nil {
		t.Fatalf("StampSettlement: %v", err)
	}
	entries, _ = s.ListEntries(ctx, ref)
	if entries[0].SettlementTime == nil {
		t.Fatal("settlement not stamped")
	}
	// No unsettled entries remain.
	if err := s.StampSettlement(ctx, ref, time.Now().UTC()); err == nil {
		t.Fatal("want ErrNotFound on second stamp")
	}
}

func TestPostgresStoreWebhookProcessing(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()
	ref := "optd-test-hubtel-" + uuid.New().String()[:8]
	req := charges.ChargeRequest{Product: "test", Wallet: "0241234567", IdempotencyKey: "wh-" + ref}
	if err := s.CreateIntent(ctx, ref, req, gateway.GatewayHubtel, money.New(1800, money.GHS), ""); err != nil {
		t.Fatalf("CreateIntent: %v", err)
	}

	// Dedup insert.
	inserted, err := s.InsertWebhookInbox(ctx, "evt-"+ref, "hubtel", []byte("{}"))
	if err != nil || !inserted {
		t.Fatalf("first insert = %v, %v; want true", inserted, err)
	}
	inserted, err = s.InsertWebhookInbox(ctx, "evt-"+ref, "hubtel", []byte("{}"))
	if err != nil || inserted {
		t.Fatalf("duplicate insert = %v, %v; want false", inserted, err)
	}

	// FindIntent.
	intent, err := s.FindIntent(ctx, ref)
	if err != nil || intent.Status != "pending" || intent.Product != "test" {
		t.Fatalf("FindIntent = %+v, %v", intent, err)
	}
	if _, err := s.FindIntent(ctx, "optd-missing-01ARZ3NDEKTSV4RRFFQ69G5FAV"); err == nil {
		t.Fatal("want ErrUnknownRef")
	}

	// Apply outcome transactionally: status + ledger entry.
	entry := ledger.LedgerEntry{
		ID:          uuid.New(),
		Kind:        ledger.KindCollection,
		Ref:         ref,
		Amount:      money.New(1800, money.GHS),
		ValueTime:   time.Now().UTC(),
		BookingTime: time.Now().UTC(),
		Product:     "test",
	}
	if err := s.ApplyChargeOutcome(ctx, ref, "succeeded", []ledger.LedgerEntry{entry}); err != nil {
		t.Fatalf("ApplyChargeOutcome: %v", err)
	}
	intent, err = s.FindIntent(ctx, ref)
	if err != nil || intent.Status != "succeeded" {
		t.Fatalf("intent after outcome = %+v, %v", intent, err)
	}
	entries, err := s.ListEntries(ctx, ref)
	if err != nil || len(entries) != 1 || entries[0].Amount.MinorUnits() != 1800 {
		t.Fatalf("entries = %+v, %v", entries, err)
	}
}

func TestPostgresStoreBatchReservations(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()
	batch := payouts.PayoutBatch{
		ID:        uuid.New(),
		Product:   "test",
		Status:    "pending",
		Total:     money.New(7500, money.GHS),
		CreatedAt: time.Now().UTC(),
		BatchDate: time.Now().UTC().Truncate(24 * time.Hour),
		Entries: []payouts.Entry{
			{Recipient: "0241111111", Amount: money.New(5000, money.GHS), Reference: "optd-test-moolre-A"},
			{Recipient: "0242222222", Amount: money.New(2500, money.GHS), Reference: "optd-test-moolre-B"},
		},
	}
	if err := s.CreateBatchWithReservations(ctx, batch); err != nil {
		t.Fatalf("CreateBatchWithReservations: %v", err)
	}
	got, err := s.FindBatch(ctx, batch.ID)
	if err != nil {
		t.Fatalf("FindBatch: %v", err)
	}
	if got.Total.MinorUnits() != 7500 || len(got.Entries) != 2 || got.Entries[0].Reference != "optd-test-moolre-A" {
		t.Fatalf("got = %+v", got)
	}

	// Settle one reservation: reopen entries by listing reservations directly.
	// The store assigns reservation IDs internally, so settle via a direct reservation.
	res := ledger.Reservation{
		ID:      uuid.New(),
		BatchID: batch.ID,
		Amount:  money.New(1000, money.GHS),
		Status:  "open",
	}
	if err := s.CreateReservation(ctx, res); err != nil {
		t.Fatalf("CreateReservation: %v", err)
	}
	if err := s.SettleReservation(ctx, res.ID, time.Now().UTC()); err != nil {
		t.Fatalf("SettleReservation: %v", err)
	}
	// Idempotent re-settle succeeds.
	if err := s.SettleReservation(ctx, res.ID, time.Now().UTC()); err != nil {
		t.Fatalf("idempotent SettleReservation: %v", err)
	}
	// Releasing an already-settled reservation is an idempotent no-op.
	if err := s.ReleaseReservation(ctx, res.ID); err != nil {
		t.Fatalf("idempotent ReleaseReservation: %v", err)
	}

	missing := uuid.New()
	if err := s.ReleaseReservation(ctx, missing); err == nil {
		t.Fatal("want ErrNotFound for missing reservation")
	}
	// FindBatch for a nonexistent batch.
	if _, err := s.FindBatch(ctx, missing); err == nil {
		t.Fatal("want ErrNotFound for missing batch")
	}
	fmt.Fprintln(os.Stderr, "note: released/settled checks complete")
}
