# Local Development — Orcta Pay

Run the full system on your machine with host Postgres/Valkey (or Docker) and test the dashboard. The `blocks.so` Tailwind + shadcn PoC lives on `chore/blocks-so-poc`.

## Stack

*Services:* API `:8080` (`cmd/api`), worker (`cmd/worker`), dashboard `:5173` (`dashboard` Vite).  
*Datastores:* Postgres `:5432`, Valkey `:6379`.  
*Migrations:* `migrations/*.sql` → version `7`.

## Prerequisites

```bash
go version    # >=1.22 (repo uses 1.26)
node -v && pnpm -v  # pnpm 10.29.1
psql --version && valkey-cli ping || redis-cli ping
task --version || go install github.com/go-task/task/v3/cmd/task@latest
which migrate || go install -tags postgres github.com/golang-migrate/migrate/v4/cmd/migrate@latest
```

Host services already on this box (checked `ss -tulpn`):

```
0.0.0.0:5432  postgres
127.0.0.1:6379 valkey  PONG
```

If you prefer Docker, `task dev:up` starts `docker compose` Postgres 17 + Valkey 8. If ports collide, set `POSTGRES_PORT`/`VALKEY_PORT` in `.env` instead.

## 1. Environment

Copy examples and adjust only if your host services differ:

```bash
cp .env.example .env
cp dashboard/.env.example dashboard/.env
cat .env
# ORCTA_ENV=development
# DATABASE_URL=postgres://orcta:orcta@localhost:5432/orcta_pay?sslmode=disable
# VALKEY_ADDR=localhost:6379
# AUTH_TOKEN_SECRET=CHANGE_ME_local_development_secret_at_least_32_chars_123456
# ORCTA_PAY_API_KEY=local-dev-operator-key-12345   # operator login for dashboard
# CORS_ALLOWED_ORIGINS=http://localhost:5173,http://127.0.0.1:5173
# PAYMENTS_PRIMARY=paystack
```

`dashboard/.env` already points at the API:

```
VITE_ORCTA_PAY_URL=http://localhost:8080
VITE_ORCTA_PAY_API_KEY=pay_test_orctago_example  # product key for /v1/charges; not operator login
```

Operator login (`dashboard/src/components/AuthGate.tsx:39` → `POST /auth/login`) uses `ORCTA_PAY_API_KEY` from server `.env`. `internal/api/router.go:28` now uses `allowedOrigins()` + `AllowCredentials: true` so `fetch(..., {credentials:"include"})` from `:5173` works (override via `CORS_ALLOWED_ORIGINS`).

## 2. Database

Host Postgres has no `orcta` role or `orcta_pay` DB by default. Create them:

```bash
psql "postgres://postgres@localhost:5432/postgres?sslmode=disable" -c "CREATE DATABASE orcta_pay;"
psql "postgres://postgres@localhost:5432/postgres?sslmode=disable" -c "DO \$\$ BEGIN IF NOT EXISTS (SELECT FROM pg_roles WHERE rolname='orcta') THEN CREATE ROLE orcta WITH LOGIN SUPERUSER PASSWORD 'orcta'; END IF; END \$\$;"
psql "postgres://postgres@localhost:5432/postgres?sslmode=disable" -c "DO \$\$ BEGIN IF NOT EXISTS (SELECT FROM pg_roles WHERE rolname='orcta_pay') THEN CREATE ROLE orcta_pay WITH LOGIN PASSWORD 'orcta_pay'; END IF; END \$\$;"
```

Apply migrations:

```bash
migrate -path ./migrations -database "postgres://orcta:orcta@localhost:5432/orcta_pay?sslmode=disable" up
migrate -path ./migrations -database "postgres://orcta:orcta@localhost:5432/orcta_pay?sslmode=disable" version  # 7
# or: task migrate:up / task migrate:status (Taskfile.yml:69, uses DATABASE_URL)
```

Or use the helper:

```bash
task dev:setup:host  # creates DB/roles, copies .env if missing, migrates
```

## 3. Run the system

**One command (host Postgres/Valkey must be running):**

```bash
task dev:stack        # setup + API :8080 + worker + dashboard :5173 (logs /tmp/*.log)
task dev:stack:down   # pkill api/worker/vite
```

It does: `task dev:setup:host`, then `setsid go run ./cmd/api` → `/tmp/api.log`, `go run ./cmd/worker` → `/tmp/worker.log`, `pnpm --filter orcta-pay-dashboard dev` → `/tmp/dashboard.log`, then probes `http://localhost:8080/readyz`.

**Manual (separate terminals):**

```bash
# terminal 1 — API
set -a; source .env; set +a
go run ./cmd/api          # :8080 — task run also works (loads .env via Taskfile dotenv)

# terminal 2 — worker
set -a; source .env; set +a
go run ./cmd/worker       # task run:worker

# terminal 3 — dashboard
pnpm --filter orcta-pay-dashboard dev  # :5173, proxies /v1,/auth to API via dashboard/vite.config.ts:8
```

Verify:

```bash
curl -s http://localhost:8080/healthz  # {"status":"ok"}
curl -s http://localhost:8080/readyz   # {"checks":{"postgres":"ok","valkey":"ok"},"status":"ready"}
curl -s -H "Origin: http://localhost:5173" -i http://localhost:8080/auth/session | grep -i Access-Control
# Access-Control-Allow-Origin: http://localhost:5173
# Access-Control-Allow-Credentials: true
```

Open `http://localhost:5173` — login with `ORCTA_PAY_API_KEY` (`local-dev-operator-key-12345` by default).

## 4. Seed demo data

Fresh DB has no apps/charges (`GET /v1/apps` → `[]`). Seed:

```bash
task dev:seed          # wraps bash scripts/seed.sh
# or: bash scripts/seed.sh
# uses $API_URL (default http://localhost:8080) and $DATABASE_URL
# creates apps orctago/pos + 6 payment_intents + 6 ledger_entries + 1 payout batch (3 reservations) + 2 webhooks

curl -s http://localhost:8080/v1/apps | python3 -m json.tool
curl -s http://localhost:8080/v1/charges | python3 -m json.tool | head -n 40
```

Dashboard Overview then shows `Total processed GHS 118.00` etc. See `scripts/seed.sh:1` for direct `psql` inserts (`optd-{product}-{gateway}-{ulid}`).

## 5. blocks.so PoC (branch `chore/blocks-so-poc`)

That branch adds Tailwind v4 + `shadcn` + `blocks.so` registry (`dashboard/components.json: registries: {"@blocks-so": "https://blocks.so/r/{name}.json"}`) with demo blocks `stats-01`/`dialog-01`/`sidebar-01` on `/blocks`. To try:

```bash
git checkout chore/blocks-so-poc
pnpm install
pnpm --filter orcta-pay-dashboard run build  # 2287 modules
task dev:stack  # same stack, now with blocks
# open http://localhost:5173/ and http://localhost:5173/blocks
```

Add more: `npx shadcn@latest add @blocks-so/table-01` etc.

## 6. Useful commands

```bash
task --list
task check          # fmt, vet, lint, comments, api:lint, test:race
task test:race
task generate       # after SQL changes: sqlc generate
task migrate:new -- add_foo
psql "postgres://orcta:orcta@localhost:5432/orcta_pay?sslmode=disable" -c "SELECT * FROM payment_intents ORDER BY created_at DESC LIMIT 5;"
valkey-cli -h localhost ping
```

## 7. Troubleshooting

| Symptom | Fix |
|---|---|
| `config: required variable not set: DATABASE_URL` | `set -a; source .env; set +a` before `go run`, or use `task run` / `task dev:stack` (loads dotenv) |
| `bind: address already in use :8080/:5173` | `task dev:stack:down` or `ss -tulpn \| grep 8080`; `pkill -f "go run ./cmd/api"`; `pkill -f vite` |
| `FATAL: role "orcta" does not exist` | `task dev:setup:host` creates it, or see §2 |
| `CORS blocked: * when credentials is include` | Fixed in `internal/api/router.go:28` — `task dev:stack` restarts API with new CORS. Check `curl -i -H "Origin: http://localhost:5173" http://localhost:8080/auth/session` |
| Dashboard blank / 401 | Login with `ORCTA_PAY_API_KEY` from `.env` (`local-dev-operator-key-12345`); check `localStorage orcta_pay_api_key` |

Logs: `/tmp/api.log`, `/tmp/worker.log`, `/tmp/dashboard.log`.

## 8. Stop & clean

```bash
task dev:stack:down
# or: pkill -f "go run ./cmd/api"; pkill -f "go run ./cmd/worker"; pkill -f "vite"
# docker: task dev:down
# reset DB: task dev:setup:host && task dev:seed  # or manual psql DROP/CREATE + migrate up
```
