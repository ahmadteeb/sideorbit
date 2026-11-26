#!/usr/bin/env bash
# Nightly Postgres backup. Keeps 14 days. Cron: 0 3 * * * /path/to/orbit/deploy/backup.sh
set -euo pipefail
cd "$(dirname "$0")"
mkdir -p backups
ts="$(date -u +%Y%m%dT%H%M%SZ)"
docker compose exec -T postgres pg_dump -U orbit -d orbit --format=custom > "backups/orbit-${ts}.dump"
find backups -name 'orbit-*.dump' -mtime +14 -delete
echo "Backup written: backups/orbit-${ts}.dump"
