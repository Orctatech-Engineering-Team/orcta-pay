package postgres

import (
	"context"
	"sync"
	"time"

	"github.com/google/uuid"

	"github.com/orctatech/orcta-pay/internal/charges"
	"github.com/orctatech/orcta-pay/internal/gateway"
	"github.com/orctatech/orcta-pay/internal/ledger"
	"github.com/orctatech/orcta-pay/internal/money"
	"github.com/orctatech/orcta-pay/internal/payouts"
)

// Store is an in-memory stub satisfying multiple domain seams.
// Replace with sqlc-backed implementation when wiring Postgres.
type Store struct {
	mu       sync.Mutex
	intents  map[string]charges.ChargePending
	idemKeys map[string]string // product:key -> ref
	ledgers  map[string][]ledger.LedgerEntry
	batches  map[uuid.UUID]payouts.PayoutBatch
}

// NewStore returns an in-memory Store.
func NewStore() *Store {
	return &Store{
		intents:  make(map[string]charges.ChargePending),
		idemKeys: make(map[string]string),
		ledgers:  make(map[string][]ledger.LedgerEntry),
		batches:  make(map[uuid.UUID]payouts.PayoutBatch),
	}
}

var (
	_ charges.IntentStore      = (*Store)(nil)
	_ payouts.ReservationStore = (*Store)(nil)
	_ ledger.Store             = (*Store)(nil)
)

// CreateIntent persists a charge intent.
func (s *Store) CreateIntent(_ context.Context, ref string, req charges.ChargeRequest, gw gateway.Gateway, _ money.Money) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.intents[ref] = charges.ChargePending{Ref: ref, Gateway: gw, ExternalRef: ref}
	if req.IdempotencyKey != "" {
		s.idemKeys[req.Product+":"+req.IdempotencyKey] = ref
	}
	return nil
}

// FindByRef returns an intent by reference.
func (s *Store) FindByRef(_ context.Context, ref string) (charges.ChargePending, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	v, ok := s.intents[ref]
	if !ok {
		return charges.ChargePending{}, charges.ErrNotFound
	}
	return v, nil
}

// FindByIdempotencyKey looks up a prior intent by product + key.
func (s *Store) FindByIdempotencyKey(_ context.Context, product, key string) (string, bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	ref, ok := s.idemKeys[product+":"+key]
	return ref, ok, nil
}

// UpdateStatus updates intent status.
func (s *Store) UpdateStatus(_ context.Context, ref string, _ string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.intents[ref]; !ok {
		return charges.ErrNotFound
	}
	return nil
}

// CreateBatchWithReservations persists a payout batch.
func (s *Store) CreateBatchWithReservations(_ context.Context, batch payouts.PayoutBatch) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.batches[batch.ID] = batch
	return nil
}

// FindBatch returns a batch by ID.
func (s *Store) FindBatch(_ context.Context, id uuid.UUID) (payouts.PayoutBatch, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	b, ok := s.batches[id]
	if !ok {
		return payouts.PayoutBatch{}, ledger.ErrNotFound
	}
	return b, nil
}

// AppendEntry appends a ledger entry.
func (s *Store) AppendEntry(_ context.Context, e ledger.LedgerEntry) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.ledgers[e.Ref] = append(s.ledgers[e.Ref], e)
	return nil
}

// ListEntries lists ledger entries by ref.
func (s *Store) ListEntries(_ context.Context, ref string) ([]ledger.LedgerEntry, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.ledgers[ref], nil
}

// CreateReservation is a no-op in the stub.
func (s *Store) CreateReservation(_ context.Context, _ ledger.Reservation) error { return nil }

// SettleReservation is a no-op in the stub.
func (s *Store) SettleReservation(_ context.Context, _ uuid.UUID, _ time.Time) error { return nil }

// ReleaseReservation is a no-op in the stub.
func (s *Store) ReleaseReservation(_ context.Context, _ uuid.UUID) error { return nil }
