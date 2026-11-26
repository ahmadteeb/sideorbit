# Self-hosting Orbit

## Requirements
- A Linux VPS or home server (amd64 or arm64; 2 vCPU / 2 GB RAM is plenty to start) with Docker + Compose v2.
- A domain such as `orbit.example.com`, with an A/AAAA record pointing to the server and ports 80 + 443 open.
- OAuth apps:
  - **GitHub:** Settings → Developer settings → OAuth Apps. Callback: `https://orbit.example.com/api/auth/github/callback`
  - **Google:** Cloud Console → Credentials → OAuth client (Web). Redirect: `https://orbit.example.com/api/auth/google/callback`
- SMTP credentials for magic links (any provider).

## Install
```bash
git clone https://github.com/<you>/orbit.git && cd orbit/deploy
cp .env.example .env
# Fill in .env. Generate secrets:
openssl rand -base64 48            # JWT_SECRET
openssl rand -base64 24            # POSTGRES_PASSWORD
docker run --rm ghcr.io/<you>/orbit-server:latest orbit-server vapid-keys   # VAPID keys (Phase 1 adds this subcommand)
docker compose up -d
docker compose logs -f orbit-server
```
Check: `https://orbit.example.com/readyz` returns `{"db":"ok","anisette":"ok"}`.

## Notes
- **anisette-v3-server** uses the upstream image `dadoum/anisette-v3-server` on the **internal** network only (no published port). Its data volume holds the Apple libraries it downloads on first start, so don't delete it casually.
- Upstream publishes images for `linux/386`, `linux/amd64` and `linux/arm64/v8`. ARMv7 (for example an old Raspberry Pi) must be built locally from its Dockerfile.
- **Backups:** `deploy/backup.sh` runs `pg_dump` into `./backups` (keeps 14 days). Add it to cron: `0 3 * * * /path/orbit/deploy/backup.sh`.
- **Updates:** `docker compose pull && docker compose up -d`. Migrations run automatically on start.
