package postgres

import (
	"context"
	"sync"
	"time"

	"github.com/google/uuid"

	"github.com/Orctatech-Engineering-Team/orcta-pay/internal/apps"
	"github.com/Orctatech-Engineering-Team/orcta-pay/internal/charges"
	"github.com/Orctatech-Engineering-Team/orcta-pay/internal/gateway"
	"github.com/Orctatech-Engineering-Team/orcta-pay/internal/ledger"
	"github.com/Orctatech-Engineering-Team/orcta-pay/internal/money"
	"github.com/Orctatech-Engineering-Team/orcta-pay/internal/payouts"
	"github.com/Orctatech-Engineering-Team/orcta-pay/internal/webhooks"
)

// MemoryStore is an in-memory implementation satisfying multiple domain seams.
// Used only as a fallback when Postgres is unavailable (e.g. tests, vet);
// data does not persist. Use PostgresStore in production.
type MemoryStore struct {
	mu       sync.Mutex
	intents  map[string]intentRow
	idemKeys map[string]string // product:key -> ref
	ledgers  map[string][]ledger.LedgerEntry
	batches  map[uuid.UUID]payouts.PayoutBatch
	apps     map[uuid.UUID]apps.App
	appHash  map[string]uuid.UUID // hash -> id
	appName  map[string]uuid.UUID // name -> id
	inbox    map[string]bool      // aggregator_event_id -> seen
}

// intentRow is the in-memory intent record.
type intentRow struct {
	pending charges.ChargePending
	status  string
	product string
}

// NewMemoryStore returns an in-memory Store.
func NewMemoryStore() *MemoryStore {
	return &MemoryStore{
		intents:  make(map[string]intentRow),
		idemKeys: make(map[string]string),
		ledgers:  make(map[string][]ledger.LedgerEntry),
		batches:  make(map[uuid.UUID]payouts.PayoutBatch),
		apps:     make(map[uuid.UUID]apps.App),
		appHash:  make(map[string]uuid.UUID),
		appName:  make(map[string]uuid.UUID),
		inbox:    make(map[string]bool),
	}
}

var (
	_ charges.IntentStore      = (*MemoryStore)(nil)
	_ payouts.ReservationStore = (*MemoryStore)(nil)
	_ ledger.Store             = (*MemoryStore)(nil)
	_ apps.Store               = (*MemoryStore)(nil)
	_ webhooks.Store           = (*MemoryStore)(nil)
)

// InsertWebhookInbox records an event (memory impl).
func (s *MemoryStore) InsertWebhookInbox(_ context.Context, aggregatorEventID, _ string, _ []byte) (bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.inbox[aggregatorEventID] {
		return false, nil
	}
	s.inbox[aggregatorEventID] = true
	return true, nil
}

// DeleteWebhookInbox removes a dedup row (memory impl).
func (s *MemoryStore) DeleteWebhookInbox(_ context.Context, aggregatorEventID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.inbox, aggregatorEventID)
	return nil
}

// FindIntent looks up an intent for webhook processing (memory impl).
func (s *MemoryStore) FindIntent(_ context.Context, ref string) (webhooks.Intent, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	row, ok := s.intents[ref]
	if !ok {
		return webhooks.Intent{}, webhooks.ErrUnknownRef
	}
	return webhooks.Intent{
		Ref:     row.pending.Ref,
		Gateway: row.pending.Gateway,
		Status:  row.status,
		Product: row.product,
	}, nil
}

// ApplyChargeOutcome updates status and appends ledger entries (memory impl).
func (s *MemoryStore) ApplyChargeOutcome(_ context.Context, ref, status string, entries []ledger.LedgerEntry) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	row, ok := s.intents[ref]
	if !ok {
		return charges.ErrNotFound
	}
	row.status = status
	s.intents[ref] = row
	s.ledgers[ref] = append(s.ledgers[ref], entries...)
	return nil
}

// CreateIntent persists a charge intent.
func (s *MemoryStore) CreateIntent(_ context.Context, ref string, req charges.ChargeRequest, gw gateway.Gateway, _ money.Money) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.intents[ref] = intentRow{
		pending: charges.ChargePending{Ref: ref, Gateway: gw, ExternalRef: ref},
		status:  "pending",
		product: req.Product,
	}
	if req.IdempotencyKey != "" {
		s.idemKeys[req.Product+":"+req.IdempotencyKey] = ref
	}
	return nil
}

// FindByRef returns an intent by reference.
func (s *MemoryStore) FindByRef(_ context.Context, ref string) (charges.ChargePending, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	row, ok := s.intents[ref]
	if !ok {
		return charges.ChargePending{}, charges.ErrNotFound
	}
	return row.pending, nil
}

// FindByIdempotencyKey looks up a prior intent by product + key.
func (s *MemoryStore) FindByIdempotencyKey(_ context.Context, product, key string) (string, bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	ref, ok := s.idemKeys[product+":"+key]
	return ref, ok, nil
}

// UpdateStatus updates intent status.
func (s *MemoryStore) UpdateStatus(_ context.Context, ref string, status string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	row, ok := s.intents[ref]
	if !ok {
		return charges.ErrNotFound
	}
	row.status = status
	s.intents[ref] = row
	return nil
}

// CreateBatchWithReservations persists a payout batch.
func (s *MemoryStore) CreateBatchWithReservations(_ context.Context, batch payouts.PayoutBatch) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.batches[batch.ID] = batch
	return nil
}

// FindBatch returns a batch by ID.
func (s *MemoryStore) FindBatch(_ context.Context, id uuid.UUID) (payouts.PayoutBatch, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	b, ok := s.batches[id]
	if !ok {
		return payouts.PayoutBatch{}, ledger.ErrNotFound
	}
	return b, nil
}

// AppendEntry appends a ledger entry.
func (s *MemoryStore) AppendEntry(_ context.Context, e ledger.LedgerEntry) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.ledgers[e.Ref] = append(s.ledgers[e.Ref], e)
	return nil
}

// ListEntries lists ledger entries by ref.
func (s *MemoryStore) ListEntries(_ context.Context, ref string) ([]ledger.LedgerEntry, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.ledgers[ref], nil
}

// StampSettlement stamps settlement_time on unsettled entries (memory impl).
func (s *MemoryStore) StampSettlement(_ context.Context, ref string, t time.Time) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	entries, ok := s.ledgers[ref]
	if !ok || len(entries) == 0 {
		return ledger.ErrNotFound
	}
	stamped := false
	for i := range entries {
		if entries[i].SettlementTime == nil {
			tt := t
			entries[i].SettlementTime = &tt
			stamped = true
		}
	}
	if !stamped {
		return ledger.ErrNotFound
	}
	return nil
}

// CreateReservation is a no-op in the stub.
func (s *MemoryStore) CreateReservation(_ context.Context, _ ledger.Reservation) error { return nil }

// SettleReservation is a no-op in the stub.
func (s *MemoryStore) SettleReservation(_ context.Context, _ uuid.UUID, _ time.Time) error {
	return nil
}

// ReleaseReservation is a no-op in the stub.
func (s *MemoryStore) ReleaseReservation(_ context.Context, _ uuid.UUID) error { return nil }

// InsertApp stores an app with its hash.
func (s *MemoryStore) InsertApp(_ context.Context, app apps.App, hash string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.appName[app.Name]; ok {
		return apps.ErrConflict
	}
	if _, ok := s.appHash[hash]; ok {
		return apps.ErrConflict
	}
	s.apps[app.ID] = app
	s.appHash[hash] = app.ID
	s.appName[app.Name] = app.ID
	return nil
}

// GetApp returns an app by id.
func (s *MemoryStore) GetApp(_ context.Context, id uuid.UUID) (apps.App, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	a, ok := s.apps[id]
	if !ok {
		return apps.App{}, apps.ErrNotFound
	}
	return a, nil
}

// GetAppByHash returns an app by hash.
func (s *MemoryStore) GetAppByHash(_ context.Context, hash string) (apps.App, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	id, ok := s.appHash[hash]
	if !ok {
		return apps.App{}, apps.ErrNotFound
	}
	return s.apps[id], nil
}

// ListApps returns all apps.
func (s *MemoryStore) ListApps(_ context.Context) ([]apps.App, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]apps.App, 0, len(s.apps))
	for _, a := range s.apps {
		out = append(out, a)
	}
	return out, nil
}

// UpdateAppKeyHash rotates the stored hash and prefix.
func (s *MemoryStore) UpdateAppKeyHash(_ context.Context, id uuid.UUID, newHash, newPrefix string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	a, ok := s.apps[id]
	if !ok {
		return apps.ErrNotFound
	}
	// Remove old hash entry.
	for h, oid := range s.appHash {
		if oid == id {
			delete(s.appHash, h)
			break
		}
	}
	if _, ok := s.appHash[newHash]; ok {
		return apps.ErrConflict
	}
	a.Prefix = newPrefix
	s.apps[id] = a
	s.appHash[newHash] = id
	return nil
}

// RevokeApp marks revoked_at.
func (s *MemoryStore) RevokeApp(_ context.Context, id uuid.UUID) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	a, ok := s.apps[id]
	if !ok {
		return apps.ErrNotFound
	}
	if a.RevokedAt != nil {
		return nil
	}
	now := time.Now().UTC()
	a.RevokedAt = &now
	s.apps[id] = a
	return nil
}
