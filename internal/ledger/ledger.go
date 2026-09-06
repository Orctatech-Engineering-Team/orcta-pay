// Package ledger hides double-entry bookkeeping and payout reservations.
package ledger

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"

	"github.com/orctatech/orcta-pay/internal/money"
)

var (
	ErrNotFound = errors.New("ledger: not found")
	ErrConflict = errors.New("ledger: conflict")
)

// EntryKind identifies the ledger account.
type EntryKind string

const (
	KindVendorCommission EntryKind = "platform_commission"
	KindVendorPayout     EntryKind = "vendor_payout"
	KindCollection       EntryKind = "collection"
)

// LedgerEntry is a double-entry, append-only row with three timestamps per ADR-035.
type LedgerEntry struct {
	ID             uuid.UUID
	Kind           EntryKind
	Ref            string
	Amount         money.Money
	ValueTime      time.Time
	BookingTime    time.Time
	SettlementTime *time.Time
	Product        string
	CreatedAt      time.Time
}

// Reservation is a payout reservation (open → settled/released).
type Reservation struct {
	ID        uuid.UUID
	BatchID   uuid.UUID
	Amount    money.Money
	Status    string // open | settled | released
	CreatedAt time.Time
}

// Store is the ledger persistence seam.
type Store interface {
	AppendEntry(ctx context.Context, e LedgerEntry) error
	ListEntries(ctx context.Context, ref string) ([]LedgerEntry, error)
	StampSettlement(ctx context.Context, ref string, t time.Time) error
	CreateReservation(ctx context.Context, r Reservation) error
	SettleReservation(ctx context.Context, id uuid.UUID, settlementTime time.Time) error
	ReleaseReservation(ctx context.Context, id uuid.UUID) error
}

// Service orchestrates ledger operations.
type Service struct {
	store Store
}

// NewService builds a Service.
func NewService(store Store) *Service { return &Service{store: store} }

// RecordCollection appends a collection entry.
func (s *Service) RecordCollection(ctx context.Context, ref, product string, amount money.Money) error {
	return s.store.AppendEntry(ctx, LedgerEntry{
		ID:          uuid.New(),
		Kind:        KindCollection,
		Ref:         ref,
		Amount:      amount,
		ValueTime:   time.Now().UTC(),
		BookingTime: time.Now().UTC(),
		Product:     product,
		CreatedAt:   time.Now().UTC(),
	})
}

// ConfirmSettlement stamps settlement_time on all unsettled entries for ref.
// Per ADR-035 the three timestamps are: value (when it happened), booking
// (when recorded), settlement (when funds moved). This closes the third.
func (s *Service) ConfirmSettlement(ctx context.Context, ref string) error {
	entries, err := s.store.ListEntries(ctx, ref)
	if err != nil {
		return err
	}
	if len(entries) == 0 {
		return ErrNotFound
	}
	return s.store.StampSettlement(ctx, ref, time.Now().UTC())
}
