# Backup & Restore Runbook — orcta-pay Postgres

`deploy/data/docker-compose.yml` runs Postgres 17 with **WAL archiving** and a **nightly `pg_dump --format=custom` sidecar**. This document is the restore procedure and is linked from `README.md#Deployment`.

## Summary

| Item | Value |
|---|---|
| **RPO** | 60s with WAL archiving (`archive_timeout=60`), 24h without S3 (dump only) |
| **RTO** | 10–30 min (dump restore), 30–60 min (PITR) |
| **Retention** | 7 daily + 4 weekly (Sunday) — see `backup/backup.sh` |
| **Encryption** | `openssl aes-256-cbc -pbkdf2` if `BACKUP_ENCRYPTION_KEY` set; bucket SSE otherwise |
| **Off-host** | `BACKUP_S3_BUCKET` + `AWS_*` (supports Tigris/S3/MinIO via `AWS_ENDPOINT`); alternative `WALG_S3_PREFIX` with `wal-g` |
| **Volumes** | `postgres_data`, `postgres_backups` (dumps), `postgres_wal_archive` (WAL segments) |

> Production checklist: set `POSTGRES_PASSWORD`, `BACKUP_S3_BUCKET`, `AWS_*`, `BACKUP_ENCRYPTION_KEY`, and `ORCTA_ENV=production` in `deploy/data/.env`. Verify with `BACKUP.md#verification`.

## Architecture

```
postgres:17-alpine (wal_level=replica, archive_mode=on, archive_timeout=60)
  archive_command: test ! -f /wal-archive/%f && cp %p /wal-archive/%f
  volumes: postgres_data:/var/lib/postgresql/data, postgres_wal_archive:/wal-archive

postgres-backup sidecar (postgres:17-alpine, depends_on postgres healthy)
  /usr/local/bin/backup.sh  — daily 02:00 UTC pg_dump custom + retention + optional S3 + optional openssl
  /usr/local/bin/restore.sh — list / verify / restore / PITR helper
  volumes: postgres_backups:/backups, postgres_wal_archive:/wal-archive:ro
```

- **Without S3:** WAL segments and dumps stay on `postgres_wal_archive` / `postgres_backups` volumes. Host failure loses both DB and backups — attach a host cron that syncs `/var/lib/docker/volumes/...` to S3 or enable `BACKUP_S3_BUCKET`.
- **With S3/Tigris (`WALG_S3_PREFIX` or `BACKUP_S3_BUCKET`):** WAL-G can replace the local `archive_command` with `wal-g wal-push %p` for continuous archiving and `wal-g backup-push` for base backups. The compose ships the simple `cp` archiver so PITR works without WAL-G; swap `archive_command` to WAL-G when `WALG_S3_PREFIX` is set (see env.example).

## Backup

### Nightly dump (sidecar)

The `postgres-backup` service loops until next 02:00 UTC and runs `backup/backup.sh`:

```bash
docker compose -p orcta-pay-data --env-file deploy/data/.env up -d postgres-backup
docker logs -f orcta-pay-postgres-backup
ls -lh $(docker volume inspect orcta-pay-data_postgres_backups --format '{{.Mountpoint}}')
```

Manual run:

```bash
docker compose -p orcta-pay-data --env-file deploy/data/.env exec postgres-backup /usr/local/bin/backup.sh
# or on host (psql available):
PGHOST=127.0.0.1 PGPORT=5435 PGUSER=orcta_pay PGDATABASE=orcta_pay BACKUP_DIR=/srv/backups ./deploy/data/backup/backup.sh
```

What `backup.sh` does:

1. `pg_isready` check.
2. `pg_dump --format=custom --compress=9 --no-acl --no-owner` to `$BACKUP_DIR/orcta_pay-YYYY-MM-DD.dump[.enc]`.
3. If `BACKUP_ENCRYPTION_KEY` set, `openssl enc -aes-256-cbc -pbkdf2 -salt` (file becomes `.dump.enc`).
4. `pg_restore --list` verify (skipped for encrypted dumps — decrypt first).
5. If `BACKUP_S3_BUCKET`/`WALG_S3_PREFIX` set, `aws s3 cp` (or `rclone`/`wal-g`) to `s3://$BUCKET/orcta-pay/postgres/`.
6. Retention: keep 7 daily; for older keep only Sundays up to 4 weeks; prune local `/wal-archive` older than 7d (S3 is authoritative).

### Host cron alternative

If you prefer host cron over the sidecar, use `backup/crontab.example`:

```bash
cp deploy/data/backup/crontab.example /etc/cron.d/orcta-pay-backup  # edit paths/env
crontab -l
tail -f /var/log/orcta-pay-backup.log
```

### Verify off-host

```bash
# Local
./deploy/data/backup/restore.sh --list
ls -lht /var/lib/docker/volumes/orcta-pay-data_postgres_backups/_data

# S3 / Tigris
aws --endpoint-url "$AWS_ENDPOINT" s3 ls "s3://$BACKUP_S3_BUCKET/orcta-pay/postgres/" --human-readable
# WAL-G
wal-g backup-list
wal-g wal-show
```

CI smoke check: `BACKUP_RUN_ON_START=1 docker compose -p orcta-pay-data up -d` runs one backup on start and healthcheck expects at least one `*.dump`.

## Restore

> **Never restore into the live volume without a snapshot.** Prefer restoring to a new volume/host and swapping after verification. `restore.sh` requires `CONFIRM_RESTORE=1` to overwrite the current DB.

### 1) List and verify a dump

```bash
./deploy/data/backup/restore.sh --list
./deploy/data/backup/restore.sh --verify /backups/orcta_pay-2026-09-08.dump
# encrypted:
BACKUP_ENCRYPTION_KEY=... ./deploy/data/backup/restore.sh --verify /backups/orcta_pay-2026-09-08.dump.enc
# from S3:
./deploy/data/backup/restore.sh --from-s3 s3://my-bucket/orcta-pay/postgres/orcta_pay-2026-09-08.dump  # downloads then verifies
```

### 2) Restore to a new volume (recommended)

```bash
# On the production host (or a drill host):

# a) Fetch dump locally (choose one)
aws --endpoint-url "$AWS_ENDPOINT" s3 cp "s3://$BACKUP_S3_BUCKET/orcta-pay/postgres/orcta_pay-2026-09-08.dump" /tmp/restore.dump
# or if encrypted:
aws --endpoint-url "$AWS_ENDPOINT" s3 cp "s3://$BACKUP_S3_BUCKET/orcta-pay/postgres/orcta_pay-2026-09-08.dump.enc" /tmp/restore.dump.enc
openssl enc -d -aes-256-cbc -pbkdf2 -in /tmp/restore.dump.enc -out /tmp/restore.dump -pass "pass:$BACKUP_ENCRYPTION_KEY"

# b) Create a scratch volume and restore into it
docker volume create orcta-pay-postgres-restore
docker run --rm \
  -v orcta-pay-postgres-restore:/var/lib/postgresql/data \
  -v /tmp/restore.dump:/tmp/restore.dump:ro \
  -e POSTGRES_USER=orcta_pay -e POSTGRES_PASSWORD=orcta_pay -e POSTGRES_DB=orcta_pay \
  postgres:17-alpine bash -c "
    docker-entrypoint.sh postgres &
    until pg_isready -U orcta_pay; do sleep 1; done
    pg_restore --clean --if-exists --no-owner --no-acl -U orcta_pay -d orcta_pay /tmp/restore.dump
  "

# c) Point a throwaway postgres at the restored volume and verify
docker run --rm -d --name pg-restore-verify \
  -p 5436:5432 -v orcta-pay-postgres-restore:/var/lib/postgresql/data \
  -e POSTGRES_USER=orcta_pay -e POSTGRES_PASSWORD=orcta_pay -e POSTGRES_DB=orcta_pay \
  postgres:17-alpine \
  -c wal_level=replica -c archive_mode=off

# d) Verify invariants (see below), then swap volumes or re-dump to prod
#    To swap live (downtime window):
#    docker compose -p orcta-pay-data stop postgres
#    docker volume rm orcta-pay-data_postgres_data  # only after snapshot!
#    docker volume create orcta-pay-data_postgres_data
#    docker run --rm -v orcta-pay-postgres-restore:/from -v orcta-pay-data_postgres_data:/to alpine cp -a /from/. /to/
#    docker compose -p orcta-pay-data up -d postgres

docker stop pg-restore-verify; docker volume rm orcta-pay-postgres-restore
```

### 3) In-place restore (dangerous — downtime, drops objects)

```bash
CONFIRM_RESTORE=1 ./deploy/data/backup/restore.sh /backups/orcta_pay-2026-09-08.dump
# or via compose exec:
docker compose -p orcta-pay-data exec -e CONFIRM_RESTORE=1 postgres-backup /usr/local/bin/restore.sh /backups/orcta_pay-2026-09-08.dump
```

### 4) Point-in-time recovery (PITR)

PITR replays WAL segments from `archive_command` to a target timestamp. The compose ships local WAL archiving (`/wal-archive`); with `WALG_S3_PREFIX` replace `archive_command` with `wal-g wal-push %p` and use `wal-g` for fetch.

`restore.sh --pitr` prints the full procedure and runs dry-run; execute manually:

```bash
./deploy/data/backup/restore.sh --pitr "2026-09-08 10:55:00+00"
```

Full steps (local WAL):

```bash
# 0. Stop traffic: pause Kamal Proxy / firewall, stop worker
# 1. Stop postgres
docker compose -p orcta-pay-data stop postgres

# 2. Snapshot current volume before destructive restore
docker run --rm \
  -v orcta-pay-data_postgres_data:/data -v $(pwd):/backup \
  alpine tar czf /backup/postgres_data-pre-pitr-$(date -u +%Y%m%dT%H%M%SZ).tgz -C / data

# 3. You need a base backup. Two options:
#  a) WAL-G: wal-g backup-fetch /var/lib/postgresql/data LATEST
#  b) Filesystem base backup (if no WAL-G): pg_basebackup on a running postgres
#     docker run --rm -v orcta-pay-data_postgres_data:/data postgres:17-alpine \
#       pg_basebackup -h postgres -U orcta_pay -D /data -Ft -z -P

# 4. Configure recovery
#    Mount the data volume into a helper, append to postgresql.conf:
echo "restore_command = 'cp /wal-archive/%f %p'" >> /var/lib/postgresql/data/postgresql.conf
echo "recovery_target_time = '2026-09-08 10:55:00+00'" >> /var/lib/postgresql/data/postgresql.conf
echo "recovery_target_action = 'promote'" >> /var/lib/postgresql/data/postgresql.conf
touch /var/lib/postgresql/data/recovery.signal
#    With WAL-G: restore_command = 'wal-g wal-fetch %f %p'

# 5. Start postgres, tail logs
docker compose -p orcta-pay-data start postgres
docker logs -f orcta-pay-postgres
# Watch for: "recovery stopping before commit of transaction ..." then "database system is ready to accept connections"

# 6. Verify invariants and recent ledger entry (manual QA per acceptance)
psql "postgres://orcta_pay:$POSTGRES_PASSWORD@127.0.0.1:5435/orcta_pay?sslmode=disable" -c "SELECT * FROM ledger_entries WHERE ref='optd-...' ORDER BY booking_time DESC LIMIT 5;"
# The entry inserted 5 minutes before target should now be visible; one inserted after target should not.

# 7. Remove recovery.signal (already promoted) and resume traffic
```

WAL-G variant (when `WALG_S3_PREFIX` set):

```bash
wal-g backup-fetch /var/lib/postgresql/data LATEST
echo "restore_command = 'wal-g wal-fetch %f %p'" >> /var/lib/postgresql/data/postgresql.conf
echo "recovery_target_time = '2026-09-08 10:55:00+00'" >> /var/lib/postgresql/data/postgresql.conf
touch /var/lib/postgresql/data/recovery.signal
```

## Verify ledger invariants after restore

The ledger is append-only double-entry per `migrations/000003_create_ledger_entries.up.sql` and reservations per `migrations/000004_create_payout_reservations.up.sql`. After any restore, run:

```sql
-- Row counts (compare to pre-restore snapshot)
SELECT 'ledger_entries' AS tbl, count(*) FROM ledger_entries
UNION ALL SELECT 'vendor_ledger_entries', count(*) FROM vendor_ledger_entries
UNION ALL SELECT 'platform_commission_entries', count(*) FROM platform_commission_entries
UNION ALL SELECT 'payout_reservations', count(*) FROM payout_reservations
UNION ALL SELECT 'payout_batches', count(*) FROM payout_batches;

-- No orphan reservations (every reservation references an existing batch)
SELECT count(*) AS orphan_reservations
FROM payout_reservations r LEFT JOIN payout_batches b ON r.batch_id=b.id
WHERE b.id IS NULL;

-- No open reservations that should have been settled/released (manual review)
SELECT status, count(*) FROM payout_reservations GROUP BY status;

-- Double-entry: sum per ref should net correctly (collections positive, payouts negative depending on kind)
-- Adjust sign per your product mapping; this catches missing half-entries:
SELECT ref, sum(amount_pesewas) AS net FROM ledger_entries GROUP BY ref HAVING sum(amount_pesewas) <> 0 LIMIT 20;

-- WAL replay check: max booking_time should be ~ target time for PITR
SELECT max(booking_time) FROM ledger_entries;
```

App-level drill (run on a host with the restored DB):

```bash
DATABASE_URL=postgres://orcta_pay:orcta_pay@localhost:5435/orcta_pay?sslmode=disable go test ./internal/storage/postgres -run TestLedger -count=1 -race
# or the full suite:
go test ./... -run TestLedgerInvariants  # if added per issue; otherwise manual SQL above
```

For the acceptance QA "insert then PITR -5 min":

```bash
psql $DATABASE_URL -c "INSERT INTO ledger_entries (kind, ref, amount_pesewas, currency, value_time, booking_time, product) VALUES ('collection', 'optd-pitr-test-$(date +%s)', 100, 'GHS', now(), now(), 'orctago');"
# Note time T, then:
./deploy/data/backup/restore.sh --pitr "$(date -u -d '5 minutes ago' '+%Y-%m-%d %H:%M:%S+00')"
# After replay, SELECT should show the test row if PITR target is after T, hide it if before T.
```

## Guardrail: `down -v` protection

`docker compose down -v` deletes `postgres_data`, `postgres_backups`, and `postgres_wal_archive` volumes — unrecoverable without S3.

- **Use `task data:down` (no `-v`)** for normal stops. It wraps `down` without volumes.
- **Use `scripts/guard.sh`** as a drop-in `docker compose` wrapper:

```bash
./deploy/data/scripts/guard.sh down -v          # blocked
ORCTA_ENV=production ./deploy/data/scripts/guard.sh down -v  # blocked with instructions
ALLOW_DATA_LOSS=1 ./deploy/data/scripts/guard.sh down -v     # explicit, allowed
./deploy/data/scripts/guard.sh --allow-data-loss down -v     # allowed
```

Add a shell alias on prod hosts:

```bash
alias dc-data='ORCTA_ENV=production ./deploy/data/scripts/guard.sh'
```

Taskfile guard: `task data:down` is safe; `task data:down:hard` requires `ALLOW_DATA_LOSS=1` env. `task dev:reset` (root `docker compose down -v`) refuses when `ORCTA_ENV=production`.

```bash
task data:down             # safe
ALLOW_DATA_LOSS=1 task data:down:hard   # requires explicit var
ORCTA_ENV=production task dev:reset     # refuses
```

## Monthly restore drill

- [ ] Restore latest dump to ephemeral host/volume, run invariant SQL and `go test ./internal/storage/postgres -run TestLedger -count=1`.
- [ ] Verify S3 listing shows 7 daily + 4 weekly objects and that the newest decrypts/verifies with `pg_restore --list`.
- [ ] For WAL archiving, perform a PITR to `-5 minutes` and confirm the recently inserted ledger entry reappears per steps above; record result in ops log.
- [ ] Rotate `BACKUP_ENCRYPTION_KEY` annually; re-encrypt oldest weekly dump.

## Troubleshooting

| Symptom | Cause | Fix |
|---|---|---|
| `archive_command` failures in `pg_log` | `/wal-archive` full or permission | `docker volume inspect`, `df -h`; prune via `backup.sh` retention; check `chown postgres` |
| `pg_dump: connection refused` | PGHOST/Port mismatch (data stack uses 5435) | Set `PGHOST=postgres` in sidecar, `127.0.0.1:5435` on host |
| `aws s3 cp` no credentials | `AWS_*` not in `.env` | Source `deploy/data/.env` and re-run; test `aws sts get-caller-identity` |
| `openssl: command not found` | Minimal postgres image | `apk add openssl` in backup sidecar or remove encryption key |
| `pg_restore: extension not found` | Dump expects `pgcrypto`/`uuid-ossp` | Pre-create extensions: `CREATE EXTENSION IF NOT EXISTS pgcrypto;` before restore |

## References

- `deploy/data/docker-compose.yml` — postgres with `wal_level=replica`, `archive_mode=on`, `postgres-backup` sidecar
- `deploy/data/backup/backup.sh` — dump, encrypt, S3, retention
- `deploy/data/backup/restore.sh` — verify, restore, PITR helper
- `deploy/data/backup/crontab.example` — host cron alternative
- `deploy/data/scripts/guard.sh` — `down -v` guardrail
- `Taskfile.yml#data:down` — safe wrappers
- `migrations/000003_create_ledger_entries.up.sql`, `000004_create_payout_reservations.up.sql` — ledger invariants
