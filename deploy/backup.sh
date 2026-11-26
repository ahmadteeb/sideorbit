#!/usr/bin/env bash
# Nightly Postgres backup. Keeps 14 days. Cron: 0 3 * * * /path/to/orbit/deploy/backup.sh
set -euo pipefail
cd "$(dirname "$0")"
umask 077 # backups hold email addresses
mkdir -p backups
ts="$(date -u +%Y%m%dT%H%M%SZ)"
out="backups/orbit-${ts}.dump"
# Write aside first so a failed dump never looks like a good backup.
trap 'rm -f "${out}.part"' EXIT
docker compose exec -T postgres pg_dump -U orbit -d orbit --format=custom > "${out}.part"
mv "${out}.part" "$out"
find backups -name 'orbit-*.dump' -mtime +14 -delete
echo "Backup written: ${out}"
