# Production datastores

Long-lived Postgres + Valkey stack. Deploy once by hand, never by Orcta deploys:

```bash
docker compose -p orcta-pay-data --env-file deploy/data/.env up -d
docker compose -p orcta-pay-data ps
```

## Valkey durability (AOF)

`deploy/data/docker-compose.yml` runs Valkey with AOF persistence:

```
--appendonly yes --appendfsync everysec --auto-aof-rewrite-percentage 100 --save "" --maxmemory-policy noeviction
```

* `appendonly yes` + `appendfsync everysec` — every write is fsynced once per second; a crash loses at most 1s of writes instead of the entire dataset (previous `appendonly no` lost all keys on restart).
* `auto-aof-rewrite-percentage 100` — rewrites the AOF when it doubles in size, bounding disk growth.
* `--save ""` disables RDB snapshots; AOF is the sole durability mechanism.
* `noeviction` is intentional for financial data — Valkey returns errors on OOM rather than evicting idempotency locks or circuit-breaker state.

This matters because `internal/storage/valkey/locker.go` (`SETNX`/`TryAcquire`) guards `payment_outbox` dispatch and payout double-spend, and `internal/storage/valkey/health.go` stores gateway circuit-breaker state. Losing those keys after a Valkey restart allows duplicate payouts and breaker resets while a gateway is still failing.

`valkey_data:/data` is a named volume; do not run production without it. `restart: unless-stopped` keeps the container up across host reboots. Healthcheck is `valkey-cli ping`.

Verify config:

```bash
docker compose -f deploy/data/docker-compose.yml --env-file deploy/data/.env config | grep -A20 valkey
valkey-cli -h 127.0.0.1 -p 6382 info persistence  # aof_enabled:1, aof_rewrite_in_progress:0
```

## Local dev

`docker-compose.yml` at the repo root is intentionally **non-durable** (`--appendonly no --save "3600 1"`) for speed. See `docs/LOCAL_DEV.md`. Do not copy its Valkey flags to production.
