#!/bin/sh
# deploy/data/scripts/guard.sh — guardrail for `docker compose down -v` / `--volumes`
# Refuses to delete production data volumes without explicit confirmation.
# Usage:
#   ./scripts/guard.sh down -v          # blocked unless ALLOW_DATA_LOSS=1 or --allow-data-loss
#   ./scripts/guard.sh down --volumes   # same
#   ALLOW_DATA_LOSS=1 ./scripts/guard.sh down -v   # allowed (use with care)
#   ./scripts/guard.sh down             # allowed (volumes preserved)
#   task data:down                      # safe alias (no -v)
#
# Integrate as git alias or shell alias:
#   alias dc-data='ORCTA_ENV=production ./deploy/data/scripts/guard.sh'
#   task data:guard -- down -v   # same check via Taskfile

set -eu

ALLOW_FLAG=0
HAS_DOWN=0
HAS_VOLUMES=0

for arg in "$@"; do
  case "$arg" in
    --allow-data-loss) ALLOW_FLAG=1 ;;
    down) HAS_DOWN=1 ;;
    -v|--volumes) HAS_VOLUMES=1 ;;
    -v*) # combined short flags like -vf etc.
      case "$arg" in *v*) HAS_VOLUMES=1 ;; esac
      ;;
  esac
done

# Also accept env var.
if [ "${ALLOW_DATA_LOSS:-0}" = "1" ]; then
  ALLOW_FLAG=1
fi

# Only guard when both `down` and volumes flag are present.
if [ "$HAS_DOWN" -eq 1 ] && [ "$HAS_VOLUMES" -eq 1 ]; then
  # In production or when ORCTA_ENV=production, require explicit flag.
  # Default to guarding (fail closed) — treat empty ORCTA_ENV as production-like for the data stack.
  env_val="${ORCTA_ENV:-production}"
  case "$env_val" in
    production|prod|staging)
      if [ "$ALLOW_FLAG" -ne 1 ]; then
        cat >&2 <<'EOF'
Refusing to run `down -v` / `down --volumes` on the data stack.

The data stack (deploy/data) holds the production ledger (vendor_ledger_entries,
platform_commission_entries, payout_reservations). Deleting its volumes is unrecoverable
without a backup restore (see deploy/data/BACKUP.md).

If you really intend to delete data volumes:

  # one-off, explicit
  ALLOW_DATA_LOSS=1 docker compose -p orcta-pay-data -f deploy/data/docker-compose.yml down -v

  # or via Taskfile (requires typing the confirmation var)
  task data:down:hard -- ALLOW_DATA_LOSS=1

  # bypass when you truly mean it and want the flag in argv
  ./deploy/data/scripts/guard.sh --allow-data-loss down -v

For normal deploys / restarts use without -v:
  docker compose -p orcta-pay-data up -d
  task data:down        # wraps `down` without -v

See deploy/data/BACKUP.md for restore procedure.
EOF
        exit 2
      fi
      ;;
    *)
      # Non-prod: still require explicit flag for the shared guard script
      # but allow task dev:reset (root compose) without guard. The data stack
      # is production even in dev checkout, so warn.
      if [ "$ALLOW_FLAG" -ne 1 ]; then
        echo "Guard: refusing '$*' — deleting volumes requires --allow-data-loss or ALLOW_DATA_LOSS=1." >&2
        echo "Hint: task data:down preserves volumes; task data:down:hard requires ALLOW_DATA_LOSS=1." >&2
        exit 2
      fi
      ;;
  esac
fi

# Strip our synthetic flag before exec so docker compose doesn't see it.
filtered=""
for arg in "$@"; do
  case "$arg" in --allow-data-loss) continue ;; esac
  filtered="$filtered $arg"
done
# shellcheck disable=SC2086
set -- $filtered

# If no args, show help.
if [ $# -eq 0 ]; then
  echo "usage: $0 [--allow-data-loss] <docker compose args>" >&2
  exit 2
fi

# Exec docker compose with the data compose file by default if caller didn't pass -f.
has_f=0
for arg in "$@"; do
  case "$arg" in -f|--file) has_f=1 ;; esac
done
if [ "$has_f" -eq 0 ]; then
  # Default to the data stack; caller can override by passing -f explicitly.
  exec docker compose -f deploy/data/docker-compose.yml "$@"
else
  exec docker compose "$@"
fi
