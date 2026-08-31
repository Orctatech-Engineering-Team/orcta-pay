// Package payouts hides batch disbursement and reservation.
package payouts

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"

	"github.com/orctatech/orcta-pay/internal/gateway"
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
}

// PayoutBatch is a batch of disbursements.
type PayoutBatch struct {
	ID        uuid.UUID
	Product   string
	Entries   []Entry
	Status    string
	Total     money.Money
	CreatedAt time.Time
}

// CreateRequest is the API input for POST /v1/payouts.
type CreateRequest struct {
	Product        string
	Entries        []Entry
	IdempotencyKey string
}

// ReservationStore is the seam for balance + reservation Tx.
type ReservationStore interface {
	CreateBatchWithReservations(ctx context.Context, batch PayoutBatch) error
	FindBatch(ctx context.Context, id uuid.UUID) (PayoutBatch, error)
}

// Service orchestrates payout batches.
type Service struct {
	store  ReservationStore
	router *gateway.ChargerRouter
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

// Create initiates a batch, reserving funds in one Tx per ADR-035.
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
	batch := PayoutBatch{
		ID:        uuid.New(),
		Product:   req.Product,
		Entries:   req.Entries,
		Status:    "pending",
		Total:     total,
		CreatedAt: time.Now().UTC(),
	}
	if err := s.store.CreateBatchWithReservations(ctx, batch); err != nil {
		return PayoutBatch{}, fmt.Errorf("payouts: create batch: %w", err)
	}
	observability.SetEventField(ctx, "payout_batch_id", batch.ID.String())
	return batch, nil
}

// Get returns a batch by ID.
func (s *Service) Get(ctx context.Context, id uuid.UUID) (PayoutBatch, error) {
	return s.store.FindBatch(ctx, id)
}
