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
  docker compose start orbit-server
  ```
- **Updates:** `git pull && docker compose up -d --build`. Migrations run automatically on start.
