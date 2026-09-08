#!/bin/sh
# deploy/data/backup/backup.sh — nightly pg_dump --format=custom (+ optional S3 + encryption + retention)
# Runs inside postgres-backup sidecar (postgres:17-alpine) or directly on host via cron.
# Env: PGHOST, PGUSER, PGPASSWORD, PGDATABASE, BACKUP_DIR, WAL_ARCHIVE_DIR,
#      BACKUP_S3_BUCKET, AWS_* / WALG_S3_PREFIX, BACKUP_ENCRYPTION_KEY (base64 or hex),
#      BACKUP_RETENTION_DAYS (default 7), BACKUP_RETENTION_WEEKS (default 4).
set -eu

: "${PGHOST:=postgres}"
: "${PGUSER:=orcta_pay}"
: "${PGDATABASE:=orcta_pay}"
: "${BACKUP_DIR:=/backups}"
: "${BACKUP_RETENTION_DAYS:=7}"
: "${BACKUP_RETENTION_WEEKS:=4}"

TIMESTAMP="$(date -u +%Y-%m-%dT%H%M%SZ)"
DATE="$(date -u +%Y-%m-%d)"
WEEKDAY="$(date -u +%u)" # 1=Mon 7=Sun

mkdir -p "$BACKUP_DIR"

FILE="$BACKUP_DIR/orcta_pay-$DATE.dump"
TMPFILE="$FILE.tmp"

echo "[backup] $TIMESTAMP starting pg_dump $PGDATABASE@$PGHOST"

# pg_dump custom format, compressed. Requires pg_isready first.
if ! pg_isready -h "$PGHOST" -U "$PGUSER" -d "$PGDATABASE" -q 2>/dev/null; then
  echo "[backup] pg_isready failed for $PGHOST" >&2
  exit 1
fi

# Use zstd if available, else gzip (pg_dump --compress=9 handles gzip). postgres:17-alpine ships gzip.
pg_dump -h "$PGHOST" -U "$PGUSER" -d "$PGDATABASE" \
  --format=custom --compress=9 --no-acl --no-owner \
  --file="$TMPFILE"

# Optional encryption at rest: if BACKUP_ENCRYPTION_KEY is set, encrypt with openssl aes-256-cbc.
# Key is expected as raw passphrase; for production use KMS or age. Encrypted file gets .enc suffix.
ENCRYPTED=""
if [ -n "${BACKUP_ENCRYPTION_KEY:-}" ]; then
  if command -v openssl >/dev/null 2>&1; then
    echo "[backup] encrypting dump with openssl aes-256-cbc"
    openssl enc -aes-256-cbc -pbkdf2 -salt \
      -in "$TMPFILE" -out "$TMPFILE.enc" -pass "pass:$BACKUP_ENCRYPTION_KEY"
    rm -f "$TMPFILE"
    TMPFILE="$TMPFILE.enc"
    FILE="$FILE.enc"
    ENCRYPTED=" (encrypted)"
  else
    echo "[backup] WARNING: BACKUP_ENCRYPTION_KEY set but openssl not found — storing unencrypted" >&2
  fi
fi

mv "$TMPFILE" "$FILE"
SIZE="$(du -h "$FILE" | cut -f1)"
echo "[backup] wrote $FILE ($SIZE)$ENCRYPTED"

# Verify dump is readable.
if echo "$FILE" | grep -q "\.enc$"; then
  echo "[backup] encrypted dump written — skipping pg_restore verification (needs key)"
else
  if pg_restore --list "$FILE" >/dev/null 2>&1; then
    echo "[backup] pg_restore --list OK"
  else
    echo "[backup] WARNING: pg_restore --list failed for $FILE" >&2
  fi
fi

# Optional off-host copy to S3/Tigris. Supports aws-cli, rclone, or wal-g depending on image.
if [ -n "${BACKUP_S3_BUCKET:-}" ] || [ -n "${WALG_S3_PREFIX:-}" ]; then
  DEST="${WALG_S3_PREFIX:-s3://${BACKUP_S3_BUCKET}/orcta-pay/postgres}"
  # Normalise dest without trailing slash.
  DEST="$(echo "$DEST" | sed 's|/*$||')"
  BASENAME="$(basename "$FILE")"
  if command -v aws >/dev/null 2>&1 && [ -n "${BACKUP_S3_BUCKET:-}" ]; then
    echo "[backup] uploading to s3://$BACKUP_S3_BUCKET/orcta-pay/postgres/$BASENAME"
    # AWS_ENDPOINT allows Tigris / MinIO.
    if [ -n "${AWS_ENDPOINT:-}" ]; then
      aws --endpoint-url "$AWS_ENDPOINT" s3 cp "$FILE" "s3://$BACKUP_S3_BUCKET/orcta-pay/postgres/$BASENAME" || echo "[backup] S3 upload failed" >&2
    else
      aws s3 cp "$FILE" "s3://$BACKUP_S3_BUCKET/orcta-pay/postgres/$BASENAME" || echo "[backup] S3 upload failed" >&2
    fi
  elif command -v wal-g >/dev/null 2>&1; then
    echo "[backup] wal-g backup-push fallback (WALG_S3_PREFIX=$DEST)"
    # wal-g handles its own base backup; pg_dump object is extra diagnostic.
    echo "[backup] NOTE: set BACKUP_S3_BUCKET + aws-cli for pg_dump off-host copy, or configure WAL-G for base backups"
  elif command -v rclone >/dev/null 2>&1; then
    rclone copy "$FILE" "${DEST}/" || echo "[backup] rclone copy failed" >&2
  else
    echo "[backup] WARNING: BACKUP_S3_BUCKET/WALG_S3_PREFIX set but no aws/wal-g/rclone found — dump stays local at $FILE" >&2
    echo "[backup] Install aws-cli in the backup image or set host cron with aws-cli to sync $BACKUP_DIR" >&2
  fi
fi

# Retention: 7 daily + 4 weekly (keep Sunday dumps for 4 weeks).
# Keep every dump from last N days; for older dumps keep only Sundays within M weeks.

echo "[backup] applying retention: ${BACKUP_RETENTION_DAYS} daily + ${BACKUP_RETENTION_WEEKS} weekly"

# BusyBox find on alpine: use -mtime with integer days.
# Daily expiry cutoff.
if [ "$BACKUP_RETENTION_DAYS" -gt 0 ] 2>/dev/null; then
  # List candidates older than retention days.
  for f in "$BACKUP_DIR"/orcta_pay-*.dump "$BACKUP_DIR"/orcta_pay-*.dump.enc; do
    [ -f "$f" ] || continue
    # Parse date from filename: orcta_pay-YYYY-MM-DD.dump[.enc]
    bn="$(basename "$f")"
    # shellcheck disable=SC2001
    fdate="$(echo "$bn" | sed -n 's/orcta_pay-\([0-9]\{4\}-[0-9]\{2\}-[0-9]\{2\}\).*/\1/p')"
    [ -n "$fdate" ] || continue
    # Age in days: difference between today and fdate.
    fepoch="$(date -d "$fdate" +%s 2>/dev/null || date -d "$fdate 00:00:00" +%s 2>/dev/null || echo 0)"
    nowepoch="$(date +%s)"
    if [ "$fepoch" -eq 0 ]; then continue; fi
    age_days=$(( (nowepoch - fepoch) / 86400 ))
    if [ "$age_days" -le "$BACKUP_RETENTION_DAYS" ]; then
      continue
    fi
    # Beyond daily window: keep only Sundays within weekly window.
    # Determine weekday of fdate.
    wday="$(date -d "$fdate" +%u 2>/dev/null || echo 0)"
    max_age=$(( BACKUP_RETENTION_DAYS + BACKUP_RETENTION_WEEKS * 7 ))
    if [ "$wday" = "7" ] && [ "$age_days" -le "$max_age" ]; then
      echo "[backup] retain weekly $f (age ${age_days}d, Sunday)"
      continue
    fi
    echo "[backup] pruning $f (age ${age_days}d)"
    rm -f "$f"
  done
fi

# WAL archive retention: keep 7 days of WAL segments locally (S3 is authoritative if configured).
if [ -d "${WAL_ARCHIVE_DIR:-/wal-archive}" ]; then
  if [ -n "${BACKUP_S3_BUCKET:-}" ] || [ -n "${WALG_S3_PREFIX:-}" ]; then
    echo "[backup] WAL archive off-host enabled — local /wal-archive is cache"
  fi
  # Optional: prune local WAL older than 7 days to bound disk (PITR beyond that needs S3).
  find "${WAL_ARCHIVE_DIR}" -type f -mtime +"${BACKUP_RETENTION_DAYS:-7}" -print -delete 2>/dev/null || true
fi

echo "[backup] done. Local dumps:"
ls -lh "$BACKUP_DIR" 2>/dev/null | tail -20 || true
if [ -n "${BACKUP_S3_BUCKET:-}" ]; then
  echo "[backup] verify off-host: aws --endpoint-url \$AWS_ENDPOINT s3 ls s3://$BACKUP_S3_BUCKET/orcta-pay/postgres/ 2>&1 | tail -20"
fi
