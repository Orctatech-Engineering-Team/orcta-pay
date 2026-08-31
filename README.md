# Orcta Pay

Standalone payments service for Orcta products (OrctaGo, POS). Implements ADR-034 and ADR-035: one webhook per gateway, `optd-{product}-{gateway}-{ulid}` reference, reservation before payout, three-timestamp double-entry ledger, transactional outbox, and Valkey-ranked gateway routing.

Design documentation lives in [`orcta-go-docs/`](./orcta-go-docs). Start with [`ARCHITECTURE.md`](./orcta-go-docs/ARCHITECTURE.md) and [`CONVENTIONS.md`](./orcta-go-docs/CONVENTIONS.md). Decisions are recorded as ADRs in [`orcta-go-docs/docs/adr/`](./orcta-go-docs/docs/adr).

## Getting started

```bash
task tools        # install sqlc, golangci-lint, migrate, goimports
cp .env.example .env
task dev:up       # start Postgres and Valkey
task migrate:up   # apply migrations
task run          # serve on :8080
```

If `task dev:up` fails to bind a port, point `DATABASE_URL` / `VALKEY_ADDR` at the host services or set `POSTGRES_PORT` / `VALKEY_PORT` in `.env`.

Verify:

```bash
curl localhost:8080/healthz   # liveness — process is up
curl localhost:8080/readyz    # readiness — dependencies reachable
curl localhost:8080/metrics   # Prometheus exposition
```

## Common tasks

`task` lists everything. The ones used most:

| Command | Purpose |
| --- | --- |
| `task check` | Format, vet, lint, and race tests — run before pushing |
| `task test:race` | What CI runs |
| `task generate` | Regenerate sqlc query code after changing SQL |
| `task migrate:new -- add_payout_table` | Create a migration pair |
| `task dev:reset` | Destroy local data and re-migrate from scratch |
| `task run:worker` | Run background jobs (payout batch, outbox drain, reconciliation) |

## Layout

```
cmd/api/            HTTP server entrypoint — wiring only
cmd/worker/         Background jobs — wiring only

internal/
  charges/          ChargeRequest, ChargeResult (sealed), Service with Initiate + Status
  payouts/          PayoutBatch, Service (reservation → disbursement)
  gateway/          AggregatorClient (Initiate/Verify/Refund/Payout), Hubtel + Paystack adapters, ChargerRouter
  ledger/           vendor_ledger_entries, platform_commission_entries, payout_reservations
  config/           Env parsing, validated once at startup
  platform/         Composition root — builds the dependency graph
  api/              chi router, middleware, handlers (charges, payouts, webhooks)
  observability/    slog + OTel + Prometheus behind vendor-agnostic accessors
  storage/
    postgres/       sqlc-generated code + adapters, tx.go with WithTx
    valkey/         gateway health, circuit breaker
  money/            Money type — integer minor units, never float

api/                OpenAPI spec and spec.go embedding
migrations/         golang-migrate SQL files
orcta-go-docs/      Architecture, conventions, ADRs
deploy/
  data/             long-lived Postgres + Valkey stack
  app/              API + worker + migrate (Kamal blue/green)
```

## API

[`api/openapi.yaml`](./api/openapi.yaml) is the wire contract. A test walks the router and fails if the spec and the served routes disagree.

| Method | Path | Description |
| --- | --- | --- |
| `POST` | `/v1/charges` | Initiate a charge (product, amount, wallet) → `optd-...` reference |
| `GET` | `/v1/charges/{ref}/status` | Gateway truth via `GetTransactionStatus` |
| `POST` | `/v1/payouts` | Batch payout (uses reservation) |
| `POST` | `/webhooks/hubtel` | Hubtel webhook (HMAC, dedup) |
| `POST` | `/webhooks/paystack` | Paystack webhook (HMAC, dedup) |

`task api:docs` renders the reference. `task api:lint` validates the spec.

## Deployment

Pull-based through Orcta Runtime: push to `master` → CI publishes `ghcr.io/orctatech-engineering-team/orcta-pay:<sha>` → GHCR webhook → Orcta pulls and swaps traffic via Kamal Proxy blue/green. Serves `api.pay.orctatech.com`.

Datastores live in a separate long-lived compose stack (`deploy/data/`), never in the app's compose file.

```
deploy/
  data/    docker-compose.yml + env.example   → /srv/apps/orcta-pay-data/
  app/     docker-compose.yml                 → /srv/apps/orcta-pay/
```

## Observability

Domain packages never import OpenTelemetry or Prometheus directly — they use `internal/observability`:

```go
ctx, span := observability.StartSpan(ctx, "charges.Initiate")
defer span.End()
observability.SetEventField(ctx, "charge_ref", ref)
observability.LoggerFromContext(ctx).InfoContext(ctx, "charge_initiated")
```

Each request emits one wide event with every layer's fields on a single line, alongside level-based logs. Every `ERROR` should have an alert attached.
