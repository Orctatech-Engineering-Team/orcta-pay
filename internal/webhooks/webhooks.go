// Package webhooks processes gateway callbacks: dedup via webhook_inbox,
// verify with gateway truth (GetTransactionStatus), then update intent status
// and write ledger entries in one transaction.
package webhooks

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"

	"github.com/orctatech/orcta-pay/internal/gateway"
	"github.com/orctatech/orcta-pay/internal/ledger"
	"github.com/orctatech/orcta-pay/internal/observability"
)

// Errors.
var (
	// ErrDuplicate means the event was already recorded; callers ack 200.
	ErrDuplicate = errors.New("webhooks: duplicate event")
	// ErrUnknownRef means no payment intent matches the webhook reference.
	ErrUnknownRef = errors.New("webhooks: unknown charge reference")
	// ErrBadPayload means the payload lacks the fields needed to process.
	ErrBadPayload = errors.New("webhooks: malformed payload")
)

// Intent is the persisted charge intent as seen by webhook processing.
type Intent struct {
	Ref     string
	Gateway gateway.Gateway
	Status  string // pending | succeeded | failed
	Product string
}

// Store is the webhook persistence seam.
type Store interface {
	// InsertWebhookInbox records the event; returns false when the event ID
	// was already recorded (dedup).
	InsertWebhookInbox(ctx context.Context, aggregatorEventID, gatewayName string, payload []byte) (bool, error)
	// DeleteWebhookInbox removes the dedup row so a gateway retry can
	// reprocess after a transient verification failure.
	DeleteWebhookInbox(ctx context.Context, aggregatorEventID string) error
	// FindIntent looks up a charge intent by reference.
	FindIntent(ctx context.Context, ref string) (Intent, error)
	// ApplyChargeOutcome updates intent status and appends ledger entries in
	// one transaction.
	ApplyChargeOutcome(ctx context.Context, ref, status string, entries []ledger.LedgerEntry) error
}

// Service orchestrates webhook processing.
type Service struct {
	store  Store
	router *gateway.ChargerRouter
}

// NewService builds a Service.
func NewService(store Store, router *gateway.ChargerRouter) *Service {
	return &Service{store: store, router: router}
}

// Process handles one webhook payload. It is idempotent per event ID:
// duplicates return ErrDuplicate (callers ack 200 without reprocessing).
func (s *Service) Process(ctx context.Context, gw gateway.Gateway, payload []byte) error {
	ctx, span := observability.StartSpan(ctx, "webhooks.Process")
	defer span.End()

	ev, err := parsePayload(string(gw), payload)
	if err != nil {
		return fmt.Errorf("%w: %v", ErrBadPayload, err)
	}

	// Dedup: record the event first; a conflicting insert means reprocessing.
	inserted, err := s.store.InsertWebhookInbox(ctx, ev.EventID, string(gw), payload)
	if err != nil {
		return fmt.Errorf("webhooks: record event: %w", err)
	}
	if !inserted {
		return ErrDuplicate
	}

	intent, err := s.store.FindIntent(ctx, ev.Reference)
	if err != nil {
		// Unknown reference: drop the dedup row and surface ErrUnknownRef so
		// the handler can log and ack without retry churn.
		_ = s.store.DeleteWebhookInbox(ctx, ev.EventID)
		return fmt.Errorf("%w: %s", ErrUnknownRef, ev.Reference)
	}
	if intent.Status != "pending" {
		// Already terminal; nothing to do. Keep the inbox row as the audit record.
		return nil
	}

	adapter, ok := s.router.Adapter(intent.Gateway)
	if !ok {
		_ = s.store.DeleteWebhookInbox(ctx, ev.EventID)
		return fmt.Errorf("webhooks: no adapter for gateway %s", intent.Gateway)
	}
	verified, err := adapter.Verify(ctx, ev.Reference)
	if err != nil {
		// Verification failed transiently: drop the dedup row so the gateway's
		// retry reprocesses. GetTransactionStatus is the source of truth.
		_ = s.store.DeleteWebhookInbox(ctx, ev.EventID)
		return fmt.Errorf("webhooks: verify %s: %w", ev.Reference, err)
	}

	// Map verified status onto the intent. "pending" leaves the intent as-is.
	switch verified.Status {
	case "succeeded":
		entry := ledger.LedgerEntry{
			ID:          uuid.New(),
			Kind:        ledger.KindCollection,
			Ref:         ev.Reference,
			Amount:      verified.Amount,
			ValueTime:   verified.VerifiedAt,
			BookingTime: time.Now().UTC(),
			Product:     intent.Product,
		}
		if err := s.store.ApplyChargeOutcome(ctx, ev.Reference, "succeeded", []ledger.LedgerEntry{entry}); err != nil {
			return fmt.Errorf("webhooks: apply outcome: %w", err)
		}
		observability.SetEventField(ctx, "charge_ref", ev.Reference)
	case "failed":
		if err := s.store.ApplyChargeOutcome(ctx, ev.Reference, "failed", nil); err != nil {
			return fmt.Errorf("webhooks: apply outcome: %w", err)
		}
	default:
		// pending: nothing to persist yet.
	}
	return nil
}
