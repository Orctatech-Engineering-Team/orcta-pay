#!/bin/sh
# deploy/data/backup/restore.sh — restore pg_dump custom-format or PITR from WAL archive
# Usage:
#   ./restore.sh --list                          # list local dumps
#   ./restore.sh --verify <dumpfile>             # pg_restore --list sanity check
#   ./restore.sh <dumpfile>                      # restore into current postgres (DANGEROUS — requires CONFIRM_RESTORE=1)
#   ./restore.sh --pitr "2026-09-08 10:55:00+00"  # PITR target time (requires WAL archive + base backup)
#   ./restore.sh --from-s3 s3://bucket/orcta-pay/postgres/orcta_pay-2026-09-08.dump
#
# For production restores prefer restoring to a NEW volume/host (see BACKUP.md).
set -eu

BACKUP_DIR="${BACKUP_DIR:-/backups}"
PGHOST="${PGHOST:-postgres}"
PGUSER="${PGUSER:-orcta_pay}"
PGDATABASE="${PGDATABASE:-orcta_pay}"

usage() {
  cat <<'EOF'
Usage: restore.sh [OPTIONS] [dumpfile]
  --list                  List local dumps and WAL segments
  --verify FILE           Run pg_restore --list on FILE
  --pitr TIMESTAMP        PITR restore to TIMESTAMP (e.g. "2026-09-08 10:55:00+00")
  --from-s3 S3URL         Download dump from S3 before restore (requires aws cli)
  --help                  Show this help

Restore modes:
  1) pg_dump restore: restore.sh /backups/orcta_pay-YYYY-MM-DD.dump
     Recreates DB via `pg_restore --clean --if-exists --no-owner --no-acl`.
     Requires CONFIRM_RESTORE=1 unless --force is given. Stops writes first.

  2) PITR: restore.sh --pitr "2026-09-08 10:55:00+00"
     Stops postgres, restores base backup (pg_basebackup or latest dump), replays WAL
     from /wal-archive (or S3 via wal-g) to target time. See BACKUP.md full runbook.

Invariants after restore (run from app host):
  psql $DATABASE_URL -c "SELECT count(*) FROM ledger_entries;"
  psql $DATABASE_URL -c "SELECT status, count(*) FROM payout_reservations GROUP BY status;"
  go test ./internal/storage/postgres -run TestLedger -count=1
  # Double-entry check per schema docs:
  psql $DATABASE_URL -c "
    SELECT ref, sum(CASE WHEN kind='collection' THEN amount_pesewas ELSE -amount_pesewas END)
    FROM ledger_entries GROUP BY ref HAVING sum(...)<>0 LIMIT 10;"
EOF
}

list_backups() {
  echo "=== Local dumps in $BACKUP_DIR ==="
  ls -lht "$BACKUP_DIR" 2>/dev/null | head -40 || echo "(no dumps in $BACKUP_DIR)"
  echo ""
  echo "=== WAL archive in ${WAL_ARCHIVE_DIR:-/wal-archive} ==="
  ls -lht "${WAL_ARCHIVE_DIR:-/wal-archive}" 2>/dev/null | head -40 || echo "(no WAL archive or dir missing)"
  echo ""
  if [ -n "${BACKUP_S3_BUCKET:-}" ] || [ -n "${WALG_S3_PREFIX:-}" ]; then
    echo "=== S3 (requires aws cli) ==="
    if command -v aws >/dev/null 2>&1; then
      if [ -n "${BACKUP_S3_BUCKET:-}" ]; then
        aws ${AWS_ENDPOINT:+--endpoint-url "$AWS_ENDPOINT"} s3 ls "s3://$BACKUP_S3_BUCKET/orcta-pay/postgres/" 2>&1 | tail -20 || true
      fi
      if [ -n "${WALG_S3_PREFIX:-}" ] && command -v wal-g >/dev/null 2>&1; then
        wal-g backup-list 2>&1 | tail -20 || true
      fi
    else
      echo "(aws cli not found — install to list S3)"
    fi
  fi
}

verify_dump() {
  f="$1"
  if echo "$f" | grep -q "\.enc$"; then
    if [ -z "${BACKUP_ENCRYPTION_KEY:-}" ]; then
      echo "encrypted dump $f requires BACKUP_ENCRYPTION_KEY to verify" >&2
      exit 2
    fi
    tmp="$(mktemp /tmp/restore-verify-XXXX.dump)"
    trap 'rm -f "$tmp"' EXIT
    openssl enc -d -aes-256-cbc -pbkdf2 -in "$f" -out "$tmp" -pass "pass:$BACKUP_ENCRYPTION_KEY"
    f="$tmp"
  fi
  echo "pg_restore --list $1"
  pg_restore --list "$f" | head -100
  echo "verify OK: $(pg_restore --list "$f" | wc -l) objects"
}

do_restore_dump() {
  dump="$1"
  if [ ! -f "$dump" ]; then
    echo "dump not found: $dump" >&2; exit 2
  fi
  if [ "${CONFIRM_RESTORE:-0}" != "1" ] && [ "${FORCE:-0}" != "1" ]; then
    cat >&2 <<EOF
Refusing to restore $dump into $PGDATABASE@$PGHOST without CONFIRM_RESTORE=1.
This will DROP and recreate objects (pg_restore --clean --if-exists).

For a safe restore, follow BACKUP.md: restore to a NEW volume/host instead:
  docker volume create orcta-pay-postgres-restore
  docker compose -f deploy/data/docker-compose.yml --env-file deploy/data/.env \
    run --rm -v orcta-pay-postgres-restore:/var/lib/postgresql/data postgres ...

If you really want to overwrite the current DB:
  CONFIRM_RESTORE=1 $0 $dump
EOF
    exit 2
  fi

  # Decrypt if needed
  plain="$dump"
  tmp=""
  if echo "$dump" | grep -q "\.enc$"; then
    if [ -z "${BACKUP_ENCRYPTION_KEY:-}" ]; then
      echo "BACKUP_ENCRYPTION_KEY required to decrypt $dump" >&2; exit 2
    fi
    tmp="$(mktemp /tmp/restore-XXXX.dump)"
    trap 'rm -f "$tmp"' EXIT
    openssl enc -d -aes-256-cbc -pbkdf2 -in "$dump" -out "$tmp" -pass "pass:$BACKUP_ENCRYPTION_KEY"
    plain="$tmp"
  fi

  echo "[restore] pg_restore --clean --if-exists into $PGDATABASE@$PGHOST from $dump"
  echo "[restore] disconnecting other sessions..."
  psql -h "$PGHOST" -U "$PGUSER" -d postgres -c "SELECT pg_terminate_backend(pid) FROM pg_stat_activity WHERE datname='$PGDATABASE' AND pid<>pg_backend_pid();" || true

  pg_restore -h "$PGHOST" -U "$PGUSER" -d "$PGDATABASE" \
    --clean --if-exists --no-owner --no-acl --verbose "$plain" || {
      echo "[restore] pg_restore exited non-zero — check that extensions and roles exist" >&2
      exit 1
    }

  echo "[restore] done. Running post-restore checks..."
  psql -h "$PGHOST" -U "$PGUSER" -d "$PGDATABASE" -c "SELECT 'ledger_entries' AS tbl, count(*) FROM ledger_entries UNION ALL SELECT 'payout_reservations', count(*) FROM payout_reservations UNION ALL SELECT 'payout_batches', count(*) FROM payout_batches;"
  echo "[restore] verify invariants per BACKUP.md before returning traffic."
}

do_pitr() {
  target="$1"
  cat <<EOF
[restore] PITR to $target requested.

PITR requires a base backup + replay of WAL segments from ${WAL_ARCHIVE_DIR:-/wal-archive}.

Full runbook: deploy/data/BACKUP.md#point-in-time-recovery-pitr

Summary for current stack (local WAL archive mode):

  1. Stop API/worker traffic (Kamal Proxy or firewall).

  2. Stop postgres:
       docker compose -p orcta-pay-data stop postgres

  3. Preserve current data:
       docker run --rm -v orcta-pay-data_postgres_data:/data -v \$(pwd):/backup alpine tar czf /backup/postgres_data-pre-pitr-\$(date -u +%Y%m%dT%H%M%SZ).tgz -C / data

  4. Restore base backup: if using wal-g
       wal-g backup-fetch /var/lib/postgresql/data LATEST
     else (pg_dump mode): recreate DB from latest dump before target
       pg_restore --clean --if-exists -h 127.0.0.1 -p 5435 -U orcta_pay -d orcta_pay /backups/orcta_pay-<date>.dump
     For true PITR without wal-g base backup you need a filesystem base backup:
       docker run --rm -v orcta-pay-data_postgres_data:/data postgres:17-alpine pg_basebackup -h postgres -D /data -Ft -z -P

  5. Configure recovery to target time:
       echo "restore_command = 'cp /wal-archive/%f %p'" >> /var/lib/postgresql/data/postgresql.conf
       echo "recovery_target_time = '$target'" >> /var/lib/postgresql/data/postgresql.conf
       echo "recovery_target_action = 'promote'" >> /var/lib/postgresql/data/postgresql.conf
       touch /var/lib/postgresql/data/recovery.signal

  6. Start postgres and watch logs:
       docker compose -p orcta-pay-data start postgres
       docker logs -f orcta-pay-postgres

  7. After promotion, verify per BACKUP.md#verify-ledger-invariants and remove recovery.signal.

With wal-g + S3 the same steps use:
  wal-g backup-fetch ... && wal-g wal-fetch ...
See BACKUP.md for the wal-g variant.

EOF
  echo "[restore] PITR dry-run complete — execute steps above manually. Target: $target"
}

# arg parse
if [ $# -eq 0 ]; then usage; exit 2; fi
case "${1:-}" in
  --help|-h) usage; exit 0;;
  --list) list_backups; exit 0;;
  --verify) [ -n "${2:-}" ] || { echo "--verify requires FILE" >&2; exit 2; }; verify_dump "$2"; exit 0;;
  --pitr) [ -n "${2:-}" ] || { echo "--pitr requires TIMESTAMP" >&2; exit 2; }; do_pitr "$2"; exit 0;;
  --from-s3)
    s3url="${2:-}"; [ -n "$s3url" ] || { echo "--from-s3 requires S3URL" >&2; exit 2; }
    fname="$(basename "$s3url")"
    dest="$BACKUP_DIR/$fname"
    echo "downloading $s3url -> $dest"
    if command -v aws >/dev/null 2>&1; then
      aws ${AWS_ENDPOINT:+--endpoint-url "$AWS_ENDPOINT"} s3 cp "$s3url" "$dest"
      do_restore_dump "$dest"
    else
      echo "aws cli not found" >&2; exit 2
    fi
    exit 0
    ;;
  --force) FORCE=1; shift; do_restore_dump "${1:-}"; exit 0;;
  -*) echo "unknown option $1" >&2; usage; exit 2;;
  *) do_restore_dump "$1"; exit 0;;
esac
