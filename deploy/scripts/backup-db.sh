#!/usr/bin/env bash
# ============================================================================
# MAGIC — Daily PostgreSQL backup
# ----------------------------------------------------------------------------
# Dumps the database from the running Docker container, compresses it, and
# keeps the last N days. Designed to be run from cron.
#
# Install (as the 'magic' user):
#   chmod +x backup-db.sh
#   crontab -e
#   # Daily at 03:30 server time:
#   30 3 * * * /home/magic/MAGIC/deploy/scripts/backup-db.sh >> /home/magic/backups/backup.log 2>&1
#
# Optional off-site copy to Cloudflare R2 is included (commented) at the end.
# ============================================================================
set -euo pipefail

CONTAINER="magic-postgres"
ENV_FILE="$(dirname "$0")/../.env.prod"
BACKUP_DIR="${BACKUP_DIR:-$HOME/backups}"
RETENTION_DAYS="${RETENTION_DAYS:-14}"

# Load DB credentials from .env.prod
set -a; # shellcheck disable=SC1090
source "$ENV_FILE"; set +a

mkdir -p "$BACKUP_DIR"
STAMP="$(date +%Y%m%d-%H%M%S)"
OUT="$BACKUP_DIR/magicdb-$STAMP.sql.gz"

echo "[$(date)] Starting backup -> $OUT"
docker exec -t "$CONTAINER" \
  pg_dump -U "$POSTGRES_USER" -d "$POSTGRES_DB" --no-owner --clean --if-exists \
  | gzip > "$OUT"

# Integrity check: file exists and is non-trivial in size
if [[ ! -s "$OUT" ]] || [[ "$(stat -c%s "$OUT")" -lt 100 ]]; then
  echo "[$(date)] ERROR: backup looks empty, aborting." >&2
  exit 1
fi
echo "[$(date)] Backup OK ($(du -h "$OUT" | cut -f1))"

echo "[$(date)] Pruning backups older than ${RETENTION_DAYS} days"
find "$BACKUP_DIR" -name 'magicdb-*.sql.gz' -mtime +"$RETENTION_DAYS" -delete

# --- Optional: off-site copy to Cloudflare R2 (requires awscli configured) ---
# aws s3 cp "$OUT" "s3://${R2_BUCKET}-backups/$(basename "$OUT")" \
#   --endpoint-url "https://${R2_ACCOUNT_ID}.r2.cloudflarestorage.com"

echo "[$(date)] Done."
