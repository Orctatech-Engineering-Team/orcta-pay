---
title: Orcta Pay Architecture
description: Collections, payouts, ledger, and reconciliation for the standalone Orcta Pay service
last_updated: 2026-08-31
---

# Orcta Pay — Architecture

Standalone payments service for Orcta products (OrctaGo, POS). See ADR-034 and ADR-035.

## Guiding principle

Same as OrctaGo: Parnas decomposition. Each package hides one decision likely to change. Gateway adapters hide provider shape; `charges` hides reference generation; `ledger` hides double-entry; `payouts` hides reservation.

## Domain map

| Package | Secret it hides |
| --- | --- |
| `charges` | Collection lifecycle and `optd-{product}-{gateway}-{ulid}` generation |
| `payouts` | Batch disbursement and reservation before payout |
| `gateway` | Provider selection, ranking via Valkey, and per-gateway HTTP shape |
| `ledger` | Double-entry, append-only, three timestamps per entry |

`gateway.AggregatorClient` is `Initiate / Verify / Refund / Payout`. `gateway.ChargerRouter` ranks gateways via `gateway.HealthStore` (Valkey rolling metrics + circuit breaker) and fails over.

## Reference scheme

`optd-{product}-{gateway}-{ulid}` where `ulid` is a Crockford Base32 ULID. Generated at intent creation and passed as `reference`/`clientReference` on every attempt, including retries. Stored in `payment_intents.ref`.

## Webhooks

One webhook per gateway, never per product: `POST /webhooks/hubtel` and `POST /webhooks/paystack`. Each verifies HMAC with constant-time compare (`crypto/hmac`), dedups via `webhook_inbox` unique on `aggregator_event_id`, then calls `GetTransactionStatus` (`Verify`) for truth and writes ledger in same Tx. Product attribution stays in Orcta Pay, not gateway routing.

## Ledger

Double-entry, append-only, three timestamps per entry: `value_time` (event occurred), `booking_time` (recorded), `settlement_time` (funds moved, null until confirmed). Tables: `ledger_entries`, `vendor_ledger_entries`, `platform_commission_entries`. No mutable balance column; corrections are compensating entries.

## Payout reservations

`payout_reservations` (`open` → `settled`/`released`) so balance check + reservation is one linearizable transaction. Prevents two concurrent batch runs from paying same funds twice.

## Outbox

`gateway_events` with `FOR UPDATE SKIP LOCKED` for provider dispatch; `webhook_inbox` for deduplication. Standing daily reconciliation diffs provider statements vs ledger; mismatches alert, never auto-overwrite.

## Folder structure

```
cmd/api/, cmd/worker/
internal/charges, payouts, gateway, ledger, config, platform, api, observability, money, storage/postgres, storage/valkey
api/openapi.yaml
migrations/
deploy/app/, deploy/data/
```

Dependencies point one direction; `charges`/`payouts` declare interfaces, `storage/*` satisfies them. `internal/api` is thin translation.

## Tech choices

Router `chi`, DB `sqlc` + `pgx`, migrations `golang-migrate`, config plain struct, Valkey per-domain interfaces, logging `log/slog`, tracing OTel.

## Deployment

Same as OrctaGo: Kamal Proxy blue/green with `/healthz` probe, `migrate` gateway init container, datastores in separate long-lived compose project (`deploy/data/`). See `deploy/app/docker-compose.yml`.
