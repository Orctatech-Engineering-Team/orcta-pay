package postgres

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/orctatech/orcta-pay/internal/charges"
	"github.com/orctatech/orcta-pay/internal/gateway"
	"github.com/orctatech/orcta-pay/internal/ledger"
	"github.com/orctatech/orcta-pay/internal/money"
	"github.com/orctatech/orcta-pay/internal/payouts"
	"github.com/orctatech/orcta-pay/internal/webhooks"
)

// PostgresStore is the sqlc-backed implementation of the domain persistence
// seams: charges.IntentStore, payouts.ReservationStore, and ledger.Store.
// apps.Store lives in apps_store.go. Data persists across restarts.
type PostgresStore struct {
	db *DB
	q  *Queries
}

// NewPostgresStore builds a PostgresStore over a pool.
func NewPostgresStore(pool *pgxpool.Pool) *PostgresStore {
	db := NewDB(pool)
	return &PostgresStore{db: db, q: New(pool)}
}

var (
	_ charges.IntentStore      = (*PostgresStore)(nil)
	_ payouts.ReservationStore = (*PostgresStore)(nil)
	_ ledger.Store             = (*PostgresStore)(nil)
	_ webhooks.Store           = (*PostgresStore)(nil)
)

func isUniqueViolation(err error) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == "23505"
}

// InsertWebhookInbox records an event; returns false on duplicate.
func (s *PostgresStore) InsertWebhookInbox(ctx context.Context, aggregatorEventID, gatewayName string, payload []byte) (bool, error) {
	rows, err := s.q.InsertWebhookInboxDedup(ctx, InsertWebhookInboxDedupParams{
		AggregatorEventID: aggregatorEventID,
		Gateway:           gatewayName,
		Payload:           payload,
	})
	if err != nil {
		return false, fmt.Errorf("postgres: insert webhook inbox: %w", err)
	}
	return rows > 0, nil
}

// DeleteWebhookInbox removes a dedup row so a retry can reprocess.
func (s *PostgresStore) DeleteWebhookInbox(ctx context.Context, aggregatorEventID string) error {
	if _, err := s.q.DeleteWebhookInbox(ctx, aggregatorEventID); err != nil {
		return fmt.Errorf("postgres: delete webhook inbox: %w", err)
	}
	return nil
}

// FindIntent looks up a charge intent by reference for webhook processing.
func (s *PostgresStore) FindIntent(ctx context.Context, ref string) (webhooks.Intent, error) {
	row, err := s.q.GetPaymentIntentForWebhook(ctx, ref)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return webhooks.Intent{}, webhooks.ErrUnknownRef
		}
		return webhooks.Intent{}, fmt.Errorf("postgres: get intent for webhook: %w", err)
	}
	return webhooks.Intent{
		Ref:     row.Ref,
		Gateway: gateway.Gateway(row.Gateway),
		Status:  row.Status,
		Product: row.Product,
	}, nil
}

// ApplyChargeOutcome updates intent status and appends ledger entries in one
// transaction, per payments-design §5.
func (s *PostgresStore) ApplyChargeOutcome(ctx context.Context, ref, status string, entries []ledger.LedgerEntry) error {
	err := s.db.WithTx(ctx, func(ctx context.Context, tx pgx.Tx) error {
		qtx := s.q.WithTx(tx)
		if err := qtx.UpdatePaymentIntentStatus(ctx, UpdatePaymentIntentStatusParams{
			Ref:    ref,
			Status: status,
		}); err != nil {
			return err
		}
		for _, e := range entries {
			var settlement *time.Time
			if e.SettlementTime != nil {
				t := *e.SettlementTime
				settlement = &t
			}
			if err := qtx.InsertLedgerEntry(ctx, InsertLedgerEntryParams{
				ID:             e.ID,
				Kind:           string(e.Kind),
				Ref:            e.Ref,
				AmountPesewas:  e.Amount.MinorUnits(),
				Currency:       string(e.Amount.Currency()),
				ValueTime:      e.ValueTime,
				BookingTime:    e.BookingTime,
				SettlementTime: settlement,
				Product:        e.Product,
			}); err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		return fmt.Errorf("postgres: apply charge outcome: %w", err)
	}
	return nil
}

func nullString(s string) pgtype.Text {
	return pgtype.Text{String: s, Valid: s != ""}
}

// CreateIntent persists a charge intent. The UNIQUE (product, idempotency_key)
// constraint makes concurrent duplicate intents fail loudly — callers check
// idempotency before create, so this only fires on a race.
func (s *PostgresStore) CreateIntent(ctx context.Context, ref string, req charges.ChargeRequest, gw gateway.Gateway, amount money.Money) error {
	err := s.q.CreatePaymentIntent(ctx, CreatePaymentIntentParams{
		Ref:            ref,
		Product:        req.Product,
		Gateway:        string(gw),
		AmountPesewas:  amount.MinorUnits(),
		Currency:       string(amount.Currency()),
		Wallet:         nullString(req.Wallet),
		IdempotencyKey: nullString(req.IdempotencyKey),
		Status:         "pending",
	})
	if err != nil {
		return fmt.Errorf("postgres: create intent: %w", err)
	}
	return nil
}

// FindByRef returns an intent by reference.
func (s *PostgresStore) FindByRef(ctx context.Context, ref string) (charges.ChargePending, error) {
	row, err := s.q.GetPaymentIntentByRef(ctx, ref)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return charges.ChargePending{}, charges.ErrNotFound
		}
		return charges.ChargePending{}, fmt.Errorf("postgres: get intent: %w", err)
	}
	return charges.ChargePending{Ref: row.Ref, Gateway: gateway.Gateway(row.Gateway), ExternalRef: row.Ref}, nil
}

// FindByIdempotencyKey looks up a prior intent by product + key.
func (s *PostgresStore) FindByIdempotencyKey(ctx context.Context, product, key string) (string, bool, error) {
	ref, err := s.q.GetPaymentIntentByIdempotencyKey(ctx, GetPaymentIntentByIdempotencyKeyParams{
		Product:        product,
		IdempotencyKey: nullString(key),
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return "", false, nil
		}
		return "", false, fmt.Errorf("postgres: get intent by idempotency key: %w", err)
	}
	return ref, true, nil
}

// CreateBatchWithReservations persists a payout batch and its reservations in
// one transaction, per ADR-035 (balance check + reservation is atomic).
func (s *PostgresStore) CreateBatchWithReservations(ctx context.Context, batch payouts.PayoutBatch) error {
	err := s.db.WithTx(ctx, func(ctx context.Context, tx pgx.Tx) error {
		qtx := s.q.WithTx(tx)
		if err := qtx.CreatePayoutBatch(ctx, CreatePayoutBatchParams{
			ID:             batch.ID,
			Product:        batch.Product,
			TotalPesewas:   batch.Total.MinorUnits(),
			Currency:       string(batch.Total.Currency()),
			Status:         batch.Status,
			BatchDate:      batch.BatchDate,
			IdempotencyKey: nullString(batch.IdempotencyKey),
		}); err != nil {
			return err
		}
		for _, e := range batch.Entries {
			if err := qtx.CreatePayoutReservation(ctx, CreatePayoutReservationParams{
				ID:            uuid.New(),
				BatchID:       batch.ID,
				AmountPesewas: e.Amount.MinorUnits(),
				Currency:      string(e.Amount.Currency()),
				Status:        "open",
				Recipient:     nullString(e.Recipient),
				Reference:     nullString(e.Reference),
			}); err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		if isUniqueViolation(err) {
			return fmt.Errorf("postgres: create batch: %w: %w", err, ledger.ErrConflict)
		}
		return fmt.Errorf("postgres: create batch: %w", err)
	}
	return nil
}

// UpdateStatus updates intent status.
func (s *PostgresStore) UpdateStatus(ctx context.Context, ref string, status string) error {
	if err := s.q.UpdatePaymentIntentStatus(ctx, UpdatePaymentIntentStatusParams{
		Ref:    ref,
		Status: status,
	}); err != nil {
		return fmt.Errorf("postgres: update intent status: %w", err)
	}
	return nil
}

// AppendEntry appends a ledger entry (append-only, three timestamps).
func (s *PostgresStore) AppendEntry(ctx context.Context, e ledger.LedgerEntry) error {
	var settlement *time.Time
	if e.SettlementTime != nil {
		t := *e.SettlementTime
		settlement = &t
	}
	err := s.q.InsertLedgerEntry(ctx, InsertLedgerEntryParams{
		ID:             e.ID,
		Kind:           string(e.Kind),
		Ref:            e.Ref,
		AmountPesewas:  e.Amount.MinorUnits(),
		Currency:       string(e.Amount.Currency()),
		ValueTime:      e.ValueTime,
		BookingTime:    e.BookingTime,
		SettlementTime: settlement,
		Product:        e.Product,
	})
	if err != nil {
		return fmt.Errorf("postgres: insert ledger entry: %w", err)
	}
	return nil
}

// ListEntries lists ledger entries by ref.
func (s *PostgresStore) ListEntries(ctx context.Context, ref string) ([]ledger.LedgerEntry, error) {
	rows, err := s.q.ListLedgerEntriesByRef(ctx, ref)
	if err != nil {
		return nil, fmt.Errorf("postgres: list ledger entries: %w", err)
	}
	out := make([]ledger.LedgerEntry, 0, len(rows))
	for _, r := range rows {
		e := ledger.LedgerEntry{
			ID:          r.ID,
			Kind:        ledger.EntryKind(r.Kind),
			Ref:         r.Ref,
			Amount:      money.New(r.AmountPesewas, money.Currency(r.Currency)),
			ValueTime:   r.ValueTime,
			BookingTime: r.BookingTime,
			Product:     r.Product,
			CreatedAt:   r.CreatedAt,
		}
		if r.SettlementTime != nil {
			t := *r.SettlementTime
			e.SettlementTime = &t
		}
		out = append(out, e)
	}
	return out, nil
}

// StampSettlement stamps settlement_time on unsettled entries for ref.
// Returns ledger.ErrNotFound when no unsettled entries exist.
func (s *PostgresStore) StampSettlement(ctx context.Context, ref string, t time.Time) error {
	rows, err := s.q.StampLedgerSettlement(ctx, StampLedgerSettlementParams{
		Ref:            ref,
		SettlementTime: &t,
	})
	if err != nil {
		return fmt.Errorf("postgres: stamp settlement: %w", err)
	}
	if rows == 0 {
		return ledger.ErrNotFound
	}
	return nil
}

// CreateReservation persists an open payout reservation.
func (s *PostgresStore) CreateReservation(ctx context.Context, r ledger.Reservation) error {
	err := s.q.CreatePayoutReservation(ctx, CreatePayoutReservationParams{
		ID:            r.ID,
		BatchID:       r.BatchID,
		AmountPesewas: r.Amount.MinorUnits(),
		Currency:      string(r.Amount.Currency()),
		Status:        r.Status,
	})
	if err != nil {
		return fmt.Errorf("postgres: create reservation: %w", err)
	}
	return nil
}

// SettleReservation moves an open reservation to settled. Idempotent no-op if
// already settled; ErrNotFound if it does not exist.
func (s *PostgresStore) SettleReservation(ctx context.Context, id uuid.UUID, settlementTime time.Time) error {
	rows, err := s.q.SettlePayoutReservation(ctx, SettlePayoutReservationParams{
		ID:        id,
		SettledAt: &settlementTime,
	})
	if err != nil {
		return fmt.Errorf("postgres: settle reservation: %w", err)
	}
	return s.reservationRowsChecked(ctx, id, rows)
}

// ReleaseReservation moves an open reservation to released.
func (s *PostgresStore) ReleaseReservation(ctx context.Context, id uuid.UUID) error {
	rows, err := s.q.ReleasePayoutReservation(ctx, id)
	if err != nil {
		return fmt.Errorf("postgres: release reservation: %w", err)
	}
	return s.reservationRowsChecked(ctx, id, rows)
}

// reservationRowsChecked maps zero-row updates: already-transitioned
// reservations are treated as idempotent success, missing ones as ErrNotFound.
func (s *PostgresStore) reservationRowsChecked(ctx context.Context, id uuid.UUID, rows int64) error {
	if rows > 0 {
		return nil
	}
	_, err := s.q.GetPayoutReservationByID(ctx, id)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return fmt.Errorf("postgres: reservation %s: %w", id, ledger.ErrNotFound)
		}
		return fmt.Errorf("postgres: get reservation: %w", err)
	}
	return nil
}

// FindBatch returns a batch by ID with its reservations as entries.
func (s *PostgresStore) FindBatch(ctx context.Context, id uuid.UUID) (payouts.PayoutBatch, error) {
	b, err := s.q.GetPayoutBatch(ctx, id)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return payouts.PayoutBatch{}, ledger.ErrNotFound
		}
		return payouts.PayoutBatch{}, fmt.Errorf("postgres: get batch: %w", err)
	}
	rows, err := s.q.ListPayoutReservationsByBatch(ctx, id)
	if err != nil {
		return payouts.PayoutBatch{}, fmt.Errorf("postgres: list reservations: %w", err)
	}
	out := payouts.PayoutBatch{
		ID:             b.ID,
		Product:        b.Product,
		Status:         b.Status,
		Total:          money.New(b.TotalPesewas, money.Currency(b.Currency)),
		CreatedAt:      b.CreatedAt,
		BatchDate:      b.BatchDate,
		IdempotencyKey: b.IdempotencyKey.String,
		Entries:        make([]payouts.Entry, 0, len(rows)),
	}
	for _, r := range rows {
		out.Entries = append(out.Entries, payouts.Entry{
			Recipient: r.Recipient.String,
			Amount:    money.New(r.AmountPesewas, money.Currency(r.Currency)),
			Reference: r.Reference.String,
		})
	}
	return out, nil
}
