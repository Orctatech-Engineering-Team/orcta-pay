---
title: ADR-035
description: Payments hardening — outbox with idempotency key, reservation before payout, three-timestamp ledger, webhook inbox dedup
status: accepted
date: 2026-08-29
owner: backend
impact: high
supersedes: null
---

## Context

`internal/payments` already has `payment_outbox` with `FOR UPDATE SKIP LOCKED` and a simple `rider_ledger_entries` ledger. The next increment needs to handle crashes between state-change commit and aggregator dispatch, concurrent batch resumption, and auditability across products. The design in `orctago-payments-domain-design (2).md` specifies the hardening; POS arriving soon makes it worth locking now rather than after a double-pay.

## Decision

- **Outbox keyed at creation:** `payment_outbox.idempotency_key` is generated at intent creation (order confirmation or payout batch line) and passed as the gateway's `reference` on every attempt, including retries. Unique constraint at the DB, not just aggregator dedup. Short-lived Valkey `SETNX` locks guard a race between worker and manual retry.
- **Raw request/response logged:** every outbound aggregator call's verbatim payload/response is persisted per outbox entry, keyed to the intent, separate from `ChargeResult` parsing.
- **Reservation before disbursement:** `payout_reservations` (`open` → `settled`/`released`) so balance check + reservation creation is one linearizable transaction. An open reservation past the threshold is a monitored condition.
- **Ledger is double-entry, append-only, three timestamps per entry:** `value_time` (event occurred), `booking_time` (recorded), `settlement_time` (funds moved, null until confirmed). No mutable balance column. Corrections are compensating entries, not edits. Negative balances are representable (reversal after payout) and recovered via netting or explicit write-off.
- **Webhook as trigger, not truth:** `webhook_inbox` dedup by `aggregator_event_id` unique, 2xx on store, `GetTransactionStatus` for authoritative state, retry on lag. Standing daily reconciliation diffs aggregator statements vs ledger; mismatches alert, never auto-overwrite.

## Why

- Idempotency at creation survives crashes and naive retries without double-firing. Raw logging lets us reprocess after a parsing bug without replaying the call.
- Reservation prevents two concurrent batch runs from paying the same funds twice — the gap `payouts-design §4.1` names.
- Three timestamps keep reporting honest when processing is delayed (backfill vs settlement). Double-entry gives the `debits + credits + aggregator net = 0` invariant for reconciliation.
- Webhook inbox + `GetTransactionStatus` handles aggregator eventual consistency and redelivery without trusting the webhook payload directly.

## Revisit if

Outbox `idempotency_key` proves unnecessary because the aggregator's dedup alone is sufficient under load, or reservation contention shows the batch needs a different isolation level. Until then, this is the hardening that lets the same service serve Go and POS.
