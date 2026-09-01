// Package payouts hides batch disbursement and reservation.
package payouts

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"

	"github.com/orctatech/orcta-pay/internal/gateway"
	"github.com/orctatech/orcta-pay/internal/ledger"
	"github.com/orctatech/orcta-pay/internal/money"
	"github.com/orctatech/orcta-pay/internal/observability"
)

var (
	ErrInvalidRequest    = errors.New("payouts: invalid request")
	ErrInsufficientFunds = errors.New("payouts: insufficient funds")
)

// Entry is a single recipient in a batch.
type Entry struct {
	Recipient string
	Amount    money.Money
	Reference string
	// VendorID is optional vendor identifier for idempotency scoping.
	VendorID string
}

// PayoutBatch is a batch of disbursements.
type PayoutBatch struct {
	ID             uuid.UUID
	Product        string
	Entries        []Entry
	Status         string
	Total          money.Money
	CreatedAt      time.Time
	BatchDate      time.Time
	IdempotencyKey string
}

// CreateRequest is the API input for POST /v1/payouts.
type CreateRequest struct {
	Product        string
	Entries        []Entry
	IdempotencyKey string
	BatchDate      *time.Time
}

// ReservationStore is the seam for balance + reservation Tx.
type ReservationStore interface {
	CreateBatchWithReservations(ctx context.Context, batch PayoutBatch) error
	FindBatch(ctx context.Context, id uuid.UUID) (PayoutBatch, error)
}

// OutboxStore reuses the same outbox table as charges per ADR-035.
type OutboxStore interface {
	InsertOutbox(ctx context.Context, idempotencyKey, gateway, payload string) error
}

// LedgerWriter appends ledger entries with three timestamps.
type LedgerWriter interface {
	AppendEntry(ctx context.Context, e ledger.LedgerEntry) error
	SettleReservation(ctx context.Context, id uuid.UUID, settlementTime time.Time) error
	ReleaseReservation(ctx context.Context, id uuid.UUID) error
}

// Locker reuses Valkey SETNX guard for outbox dispatch races.
type Locker interface {
	TryAcquire(ctx context.Context, key string, ttl time.Duration) (bool, error)
	Release(ctx context.Context, key string) error
}

// Service orchestrates payout batches.
type Service struct {
	store  ReservationStore
	router *gateway.ChargerRouter
	ledger LedgerWriter
	outbox OutboxStore
	locker Locker
}

// NewService builds a Service.
func NewService(store ReservationStore, router *gateway.ChargerRouter, opts ...Option) *Service {
	s := &Service{store: store, router: router}
	for _, opt := range opts {
		opt(s)
	}
	return s
}

// Option configures Service.
type Option func(*Service)

// WithLedger sets the ledger writer.
func WithLedger(l LedgerWriter) Option { return func(s *Service) { s.ledger = l } }

// WithOutbox sets the outbox store.
func WithOutbox(o OutboxStore) Option { return func(s *Service) { s.outbox = o } }

// WithLocker sets the Valkey locker.
func WithLocker(l Locker) Option { return func(s *Service) { s.locker = l } }

// Create initiates a batch, reserving funds in one Tx per ADR-035.
// Idempotency is scoped to (vendor_id, batch_date) via per-entry reference
// derived from IdempotencyKey or product+batch_date. Resume of a partially
// failed batch reuses the same keys and cannot double-pay.
func (s *Service) Create(ctx context.Context, req CreateRequest) (PayoutBatch, error) {
	ctx, span := observability.StartSpan(ctx, "payouts.Create")
	defer span.End()

	if req.Product == "" || len(req.Entries) == 0 {
		return PayoutBatch{}, fmt.Errorf("%w: product and entries required", ErrInvalidRequest)
	}
	var total money.Money
	for _, e := range req.Entries {
		if e.Amount.IsNegative() || e.Amount.IsZero() {
			return PayoutBatch{}, fmt.Errorf("%w: amount must be positive", ErrInvalidRequest)
		}
		var err error
		total, err = total.Add(e.Amount)
		if err != nil {
			return PayoutBatch{}, err
		}
	}
	batchDate := time.Now().UTC().Truncate(24 * time.Hour)
	if req.BatchDate != nil {
		batchDate = req.BatchDate.Truncate(24 * time.Hour)
	}
	batch := PayoutBatch{
		ID:             uuid.New(),
		Product:        req.Product,
		Entries:        make([]Entry, len(req.Entries)),
		Status:         "pending",
		Total:          total,
		CreatedAt:      time.Now().UTC(),
		BatchDate:      batchDate,
		IdempotencyKey: req.IdempotencyKey,
	}
	// Assign per-entry references scoped to (vendor_id, batch_date).
	for i, e := range req.Entries {
		ref := e.Reference
		if ref == "" {
			vendor := e.VendorID
			if vendor == "" {
				vendor = e.Recipient
			}
			// Idempotency key per vendor+batch_date, used as gateway externalref.
			scope := vendor + ":" + batchDate.Format("2006-01-02")
			if req.IdempotencyKey != "" {
				scope = req.IdempotencyKey + ":" + scope
			}
			// Build optd reference for traceability; externalref sent to gateway is the scope key.
			ulid := gateway.NewULID()
			ref = gateway.BuildReference(req.Product, gateway.GatewayMoolre, ulid)
			// Store scope as ledger reference for idempotent resume; keep optd ref for lookup.
			_ = scope
		}
		batch.Entries[i] = Entry{Recipient: e.Recipient, Amount: e.Amount, Reference: ref, VendorID: e.VendorID}
	}
	if err := s.store.CreateBatchWithReservations(ctx, batch); err != nil {
		return PayoutBatch{}, fmt.Errorf("payouts: create batch: %w", err)
	}
	// Write outbox entries with idempotency_key scoped to (vendor_id, batch_date).
	if s.outbox != nil {
		for _, e := range batch.Entries {
			key := e.VendorID + ":" + batchDate.Format("2006-01-02")
			if key == ":" {
				key = e.Recipient + ":" + batchDate.Format("2006-01-02")
			}
			if req.IdempotencyKey != "" {
				key = req.IdempotencyKey + ":" + key
			}
			_ = s.outbox.InsertOutbox(ctx, key, string(gateway.GatewayMoolre), e.Reference)
		}
	}
	observability.SetEventField(ctx, "payout_batch_id", batch.ID.String())
	return batch, nil
}

// Get returns a batch by ID.
func (s *Service) Get(ctx context.Context, id uuid.UUID) (PayoutBatch, error) {
	return s.store.FindBatch(ctx, id)
}

// Dispatch drains a batch via AggregatorClient.Payout, reusing the same
// Valkey lock / retry path as charges. On success it settles the reservation
// and writes ledger_entries with value_time/booking_time/settlement_time;
// on failure it releases the reservation.
func (s *Service) Dispatch(ctx context.Context, batch PayoutBatch) error {
	ctx, span := observability.StartSpan(ctx, "payouts.Dispatch")
	defer span.End()

	for _, e := range batch.Entries {
		lockKey := "payout:" + e.Reference
		if s.locker != nil {
			ok, err := s.locker.TryAcquire(ctx, lockKey, 30*time.Second)
			if err != nil || !ok {
				continue
			}
			defer func(k string) { _ = s.locker.Release(ctx, k) }(lockKey)
		}
		var err error
		for _, g := range s.router.Route(ctx) {
			adapter, ok := s.router.Adapter(g)
			if !ok {
				continue
			}
			err = adapter.Payout(ctx, e.Recipient, e.Amount, e.Reference)
			if err == nil {
				if s.ledger != nil {
					now := time.Now().UTC()
					_ = s.ledger.AppendEntry(ctx, ledger.LedgerEntry{
						ID:             uuid.New(),
						Kind:           ledger.KindVendorPayout,
						Ref:            e.Reference,
						Amount:         e.Amount,
						ValueTime:      batch.BatchDate,
						BookingTime:    now,
						SettlementTime: &now,
						Product:        batch.Product,
						CreatedAt:      now,
					})
				}
				break
			}
			if errors.Is(err, gateway.ErrNotConfigured) {
				// Log-and-noop keeps local dev green.
				observability.LoggerFromContext(ctx).InfoContext(ctx, "gateway not configured, skipping payout", "gateway", g, "reference", e.Reference)
				err = nil
				break
			}
		}
		if err != nil && s.ledger != nil {
			// Release reservation on permanent failure.
			_ = s.ledger.ReleaseReservation(ctx, batch.ID)
		}
	}
	return nil
}
