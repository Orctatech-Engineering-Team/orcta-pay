# Deployment — Orcta Pay (Canonical)

This is the canonical deployment reference for Orcta Pay. Other docs link here — do not duplicate deploy instructions elsewhere. If this conflicts with another doc, this wins.

## 1. Model: trunk-continuous

One branch, `master`, is always deployable. Every merge to `master` ships to production automatically.

* No `develop` / `release` / `staging` branch
* No release PR or tag gating deploys
* No cherry-picks

Why: the deploy path already has safety (CI checks + health poll + auto-rollback + blue/green with no downtime). A release branch adds a manual step with no extra safety and hides integration until later. This matches Orcta Runtime's design: `push sha → pull → health → rollback` is the gate, not a branch name.

References: `deploy/app/orcta.yml:2` (`strategy: pull`, `routing.backend: kamal`), `orcta-runtime/docs/concepts.md` (three-step model: get image → health → point traffic), `orcta-runtime/docs/architecture.md` (Execution → `compose-deploy` → rollback to `last_healthy_sha`).

## 2. How a change ships

```
fix/foo (short-lived branch) → PR → 5 required checks pass → merge to master → CI builds ghcr.io/...:sha → GHCR registry_package webhook → Orcta pulls → Kamal blue/green swap → Caddy api.pay.orctatech.com
```

Step by step:

1. **Branch from `master`**: `git checkout -b fix/123-short-name master`. Keep it short-lived (hours/days, not weeks). See `CONVENTIONS.md` if present.

2. **Open PR against `master`**. Branch protection on `master` requires all 5 checks (see §3). Direct pushes to `master` are rejected — must go via PR.

3. **CI runs** (`.github/workflows/ci.yml`):
   * `test` — `go test -race` with Postgres 17 + Valkey 8, migrates `migrations/*.sql` (`TEST_DATABASE_URL`)
   * `lint` — `golangci-lint`
   * `dashboard` — `pnpm build`
   * `deploy-config` — validates `deploy/app/docker-compose.yml` has `web` service, no `container_name`/`ports`, and `orcta.yml` constraints
   * `migrations` — `squawk` + `lock_timeout` check (`migrations/*.up.sql:1` `SET LOCAL lock_timeout = '3s'`)
   * `docker` — **only on `push` to `master` or `v*` tag** — builds `Dockerfile` (Go 1.25 + dashboard), pushes `ghcr.io/Orctatech-Engineering-Team/orcta-pay:<sha>` (`type=sha,format=long`, `build-args: VERSION`). `pull_request` runs only the first 5 jobs (no image).

4. **Merge to `master`** (squash or merge commit; keep it linear). Merge commit SHA becomes the image tag.

5. **GHCR webhook fires**. Orcta Runtime is subscribed at org level (`https://github.com/organizations/Orctatech-Engineering-Team/settings/hooks`, event `registry_package`). No secret per `ci.yml` — `GITHUB_TOKEN` is enough.

6. **Orcta deploys** (host `/srv/apps/orcta-pay/`):
   * Runs `docker compose -f deploy/app/docker-compose.yml pull` with `DEPLOY_SHA=<sha>` (image `ghcr.io/...:${DEPLOY_SHA:?}` per `deploy/app/docker-compose.yml:14`)
   * Starts `migrate` one-shot (`/usr/local/bin/migrate -path /migrations -database $MIGRATION_DATABASE_URL up`), then `web` (API `:8085`) and `worker` (`/usr/local/bin/worker`) with `depends_on: migrate: completed_successfully`
   * Polls health `http://127.0.0.1:8085/healthz` (`deploy/app/docker-compose.yml:45`) and `/readyz` (`internal/api/router.go:37`). Interval 10s, timeout 5s. On healthy → records `last_healthy_sha`, updates Caddy route (`domain: api.pay.orctatech.com`, `container_port: 8085`). On unhealthy → auto-rolls back to previous healthy SHA, no human involved (`orcta-runtime/docs/concepts.md:108`).

Datastores are **not** in this compose. `deploy/data/docker-compose.yml` runs Postgres 17 + Valkey 8 as a separate long-lived stack (`/srv/apps/orcta-pay-data/`, volumes `postgres_data` + `valkey_data` + `postgres_wal_archive`). Never add them to `deploy/app/`. See `README.md:86` and `deploy/data/README.md`.

## 3. Branch protection

`master` is protected (`gh api .../branches/master/protection`):

* `required_status_checks.strict: true`, `contexts: ["test","lint","deploy-config","dashboard","migrations"]`
* `required_pull_request_reviews.required_approving_review_count: 0` (PR required, approval not — solo/small team)
* `allow_force_pushes: false`, `allow_deletions: false`

Verify: `gh api repos/Orctatech-Engineering-Team/orcta-pay/branches/master/protection`.

## 4. Environments

**Production only by default.** One Orcta app `orcta-pay` serving `api.pay.orctatech.com`. Continuous delivery means prod gets every trunk merge within minutes.

**Optional staging** (not required for this decision): register a second Orcta app `orcta-pay-staging` with same image, separate `compose_dir` `/srv/apps/orcta-pay-staging/`, domain `staging.pay.orctatech.com`, and separate `deploy/data` volumes or same data host with distinct DB name. It would watch the same SHA (same GHCR push) — no separate branch. Add only when you need pre-prod smoke of `POST /v1/charges` on a real host.

## 5. Tags and SDK versioning

* Deploy uses **SHA**, not tags. Pushing a Git tag `v0.2.0` also builds an image (`ci.yml: on.tags: ["v*"]`), but Orcta still deploys the SHA, not the tag.
* Use tags only for **human releases** and **Go SDK versioning** (`clients/go/orctapay/client.go:99` broke to `AmountPesewas`/`Currency` per `api/openapi.yaml`). Cutting `v0.2.0` → `git tag v0.2.0 && git push origin v0.2.0` → GitHub Release with changelog. Consumers `go get github.com/Orctatech-Engineering-Team/orcta-pay@v0.2.0`.

## 6. What not to do

* Do not push directly to `master` — will be rejected.
* Do not create a `release` branch or gate prod on a tag — adds delay, not safety.
* Do not run `docker compose down -v` on prod. Use `task data:down` (safe) and `task data:down:hard` + `ALLOW_DATA_LOSS=1` guard (`deploy/data/scripts/guard.sh`, `deploy/data/BACKUP.md`).
* Do not put `container_name` or `ports` in `deploy/app/docker-compose.yml` — breaks Kamal blue/green (`deploy-config` job fails).

## 7. Local vs prod

| Context | Command | Env | Notes |
|---|---|---|---|
| Local dev | `task dev:stack` (`docs/LOCAL_DEV.md:85`) | `.env` `ORCTA_ENV=development` | Host Postgres/Valkey or `task dev:up` Docker |
| CI | `task check` / `go test -race ./...` | `TEST_DATABASE_URL` | Migrations auto-applied |
| Prod | `git push master` → auto | `deploy/app/.env` (`ORCTA_ENV=production`, `ORCTA_PAY_API_KEY` required per `internal/config/config.go:284`) | Orcta pulls, health-gated |

## 8. References

* Orcta Runtime mental model: `orcta-runtime/docs/concepts.md`, `orcta-runtime/docs/architecture.md`
* Registering apps: `orcta-runtime/docs/how-to/add-app.md`
* Compose shapes: `orcta-runtime/docs/templates/docker-compose.yml.example` (direct) vs `docker-compose.kamal.yml.example` (this repo uses kamal)
* Pay domain design: `orcta-pay-docs/docs/adr/ADR-034` (one webhook per gateway), `ADR-035` (outbox, `SETNX`, three-timestamp ledger)
* Backup/PITR: `deploy/data/BACKUP.md` (runbook, `wal_level=replica`, `backup.sh`/`restore.sh`, `BACKUP_S3_BUCKET`)
* Local setup: `docs/LOCAL_DEV.md`
* CI workflow: `.github/workflows/ci.yml`
* Deploy compose: `deploy/app/docker-compose.yml`, `deploy/app/orcta.yml`
* Data compose: `deploy/data/docker-compose.yml`

## 9. For agents

When implementing a change:

1. Branch from `master`, name `fix/<issue>-<slug>` or `feat/<slug>`.
2. Implement vertical slice (migration → store → service → api → test). Keep `task check` green.
3. Push branch, open PR against `master`. Do not merge until 5 checks are green.
4. After merge, verify `gh run list --limit 1` is `success` and `gh api .../executions?action=compose-deploy:orcta-pay` shows `success` for that SHA. If health fails, Orcta auto-rollback already ran — check `deploy/app` logs via dashboard.

