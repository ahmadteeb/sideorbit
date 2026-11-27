# Self-hosting Orbit

## Requirements
- A Linux VPS or home server (amd64 or arm64; 2 vCPU / 2 GB RAM is plenty to start) with Docker + Compose v2.
- A domain such as `orbit.example.com`, with an A/AAAA record pointing to the server and ports 80 + 443 open.
- At least one way to sign in (any of these):
  - **GitHub:** Settings → Developer settings → OAuth Apps. Callback: `https://orbit.example.com/api/auth/github/callback`
  - **Google:** Cloud Console → Credentials → OAuth client (Web). Redirect: `https://orbit.example.com/api/auth/google/callback`
  - **Email links:** SMTP credentials from any provider (port 587 with STARTTLS, or 465).

## Install
```bash
git clone https://github.com/ahmadteeb/sideorbit.git && cd sideorbit/deploy
cp .env.example .env && chmod 600 .env
# Fill in .env. Generate secrets:
openssl rand -base64 48            # JWT_SECRET
openssl rand -base64 24            # POSTGRES_PASSWORD
docker compose build orbit-server
docker compose run --rm --no-deps orbit-server vapid-keys   # paste the two lines into .env
docker compose up -d
docker compose logs -f orbit-server
```
The server refuses to start with a clear message if `.env` is incomplete.

## Verify your install
Run these from any computer. Replace the domain.

```bash
D=https://orbit.example.com

curl -s $D/healthz                                     # ok
curl -s -w ' %{http_code}\n' $D/readyz                 # {"anisette":"ok","db":"ok"} 200

# Without a token the gateway refuses.
curl -s -o /dev/null -w '%{http_code}\n' $D/anisette/v3/client_info          # 401
curl -s -o /dev/null -w '%{http_code}\n' -X POST $D/anisette/v3/get_headers  # 401
```

Then sign in the way the desktop app will, using a one-time code:

```bash
VERIFIER=$(openssl rand -hex 32)
CHALLENGE=$(printf %s "$VERIFIER" | openssl dgst -sha256 -binary | openssl base64 -A | tr '+/' '-_' | tr -d '=')
REDIRECT=http://127.0.0.1:8765/callback

# GitHub or Google: open this in a browser (use /google/ for Google), then press "Sign in".
echo "$D/api/auth/github/start?client=desktop&code_challenge=$CHALLENGE&redirect=$REDIRECT"

# Email: request a link (expect 202), then open it from your inbox and press "Sign in".
curl -s -o /dev/null -w '%{http_code}\n' -H 'Content-Type: application/json' \
  -d "{\"email\":\"you@example.com\",\"client\":\"desktop\",\"code_challenge\":\"$CHALLENGE\",\"redirect\":\"$REDIRECT\"}" \
  $D/api/auth/email/start
```

The browser then fails to load `127.0.0.1:8765`. That is expected: copy the `code=` value from its address bar and exchange it **within 60 seconds**.

```bash
CODE=paste-the-code-here
TOKEN=$(curl -s -H 'Content-Type: application/json' \
  -d "{\"grant_type\":\"authorization_code\",\"code\":\"$CODE\",\"code_verifier\":\"$VERIFIER\"}" \
  $D/api/auth/token | sed -n 's/.*"access_token":"\([^"]*\)".*/\1/p')

curl -s -H "Authorization: Bearer $TOKEN" $D/api/me                       # your profile
curl -s -H "Authorization: Bearer $TOKEN" $D/anisette/v3/client_info      # anisette JSON

# The provisioning WebSocket: expect {"result":"GiveIdentifier"}, then it times out.
curl -s --http1.1 --max-time 4 -N -H "Authorization: Bearer $TOKEN" \
  -H 'Connection: Upgrade' -H 'Upgrade: websocket' -H 'Sec-WebSocket-Version: 13' \
  -H 'Sec-WebSocket-Key: dGhlIHNhbXBsZSBub25jZQ==' \
  $D/anisette/v3/provisioning_session | tr -c '[:print:]' ' '; echo
```

Signing in from a plain browser (`client=web`) lands on `/`, which has no page until the Phase 2 dashboard.

## Notes
- **anisette-v3-server** uses the upstream image `dadoum/anisette-v3-server`, pinned by digest, on its own network with no published port. Its data volume holds the Apple libraries it downloads on first start, so don't delete it casually.
- Upstream publishes images for `linux/386`, `linux/amd64` and `linux/arm64/v8`. ARMv7 (for example an old Raspberry Pi) must be built locally from its Dockerfile.
- **IPv6:** if you publish an AAAA record, check that rate limits see real client addresses. Without IPv6 on the Docker network, all IPv6 clients can appear as one address; enable it (`enable_ipv6: true` on the `edge` network) in that case.
- **Run one `orbit-server`.** Rate limits are held in memory.
- **Backups:** `deploy/backup.sh` runs `pg_dump` into `deploy/backups` (readable only by you, keeps 14 days). Add it to cron: `0 3 * * * /path/sideorbit/deploy/backup.sh`.
- **Restore:**
  ```bash
  docker compose stop orbit-server
  docker compose exec -T postgres pg_restore -U orbit -d orbit --clean --if-exists < backups/orbit-<timestamp>.dump
  # An old backup brings back sessions that were since signed out: end them all.
  docker compose exec -T postgres psql -U orbit -d orbit -c "UPDATE refresh_tokens SET revoked_at = now() WHERE revoked_at IS NULL; DELETE FROM push_subscriptions;"
  # Change JWT_SECRET in .env as well, then:
  docker compose up -d orbit-server
  ```
- **Updates:** `git pull && docker compose up -d --build`. Migrations run automatically on start.

Accounts and devices deleted after a backup was taken come back with it; delete them again.

## Monitoring
- **Uptime:** run `deploy/uptime-check.sh https://orbit.example.com` from **another** machine every few minutes. It exits non-zero when the server, database or anisette is down.
- **Metrics:** set `METRICS_ADDR=:9090` in `.env`. The port is not published; scrape `orbit-server:9090/metrics` from a Prometheus container on the stack's network. It listens on every container network the server is on, anisette's included; it holds counts only.
  Useful alerts:
  - `time() - orbit_scheduler_last_success_timestamp_seconds > 3600`: expiry warnings have stopped.
  - `rate(orbit_http_requests_total{status=~"5.."}[5m]) > 0`
  - `rate(orbit_http_requests_total{status="429"}[5m])`: someone is hitting limits.
  - `rate(orbit_push_notifications_total{result="failed"}[1h])`
- **Logs:** `docker compose logs -f orbit-server` (JSON). No tokens, request bodies or client addresses.

## Load testing
Only against your own server, with a token from "Verify your install" and the limits raised for the test (all from one address otherwise hits the per-IP limit):
```bash
docker run --rm -i --network host -v "$PWD/loadtest:/lt:ro" grafana/k6 run -e BASE=https://orbit.example.com -e TOKEN="$TOKEN" -e RATE=100 /lt/gateway.js
```
The script never makes anisette contact Apple. Don't change it to.

## Troubleshooting
| Symptom | Cause and fix |
|---|---|
| `orbit-server` restarts, log says `config: …` | The message names the setting in `.env` that is wrong or missing |
| `/readyz` shows `"anisette":"error"` | First start downloads Apple libraries; wait a minute, then `docker compose logs anisette-v3`. The server needs outbound internet |
| `/readyz` shows `"db":"error"` | `docker compose logs postgres`. A changed `POSTGRES_PASSWORD` doesn't change an existing database's password |
| Browser certificate warning | DNS doesn't point here yet, or ports 80/443 are blocked: `docker compose logs caddy` |
| Sign-in email never arrives | `could not send sign-in email` in the log. Check `SMTP_*`; the server must offer STARTTLS (587) or TLS (465) |
| GitHub or Google says the redirect doesn't match | The callback URL must be exactly `PUBLIC_URL/api/auth/<provider>/callback` |
| Sign-in ends with `account_exists` | That email already has an account made another way (D25). Use the original method or an email link |
| "Alerts aren't set up on this server" | Set `VAPID_PUBLIC_KEY`, `VAPID_PRIVATE_KEY` and `VAPID_SUBJECT` |

## Releasing (maintainers)
1. In GitHub → Settings → Environments, create `release` and add yourself as a required reviewer.
2. Dry run: Actions → release → Run workflow. Everything builds; nothing is published.
3. Publish: tag a commit on `main` as `vX.Y.Z` and push the tag. It publishes the multi-arch server image to ghcr and a GitHub Release with `Orbit.ipa` and checksums.
