# API contract (v1 draft)

Base URL: `https://<ORBIT_DOMAIN>`. JSON everywhere. Auth uses `Authorization: Bearer <access_jwt>`. Errors use `{"error": {"code": "string", "message": "human text", "fix": "optional human hint"}}`. Times are RFC 3339 UTC. IDs are UUIDv7 strings.

## Auth

| Method | Path | Notes |
|---|---|---|
| GET | `/api/auth/{github\|google}/start?client=web\|ios\|desktop&code_challenge=&redirect=` | Sets the `state` cookie and redirects to the provider |
| GET | `/api/auth/{provider}/callback` | Web: sets the session and redirects to `/`. ios/desktop: redirects to `orbit://auth/callback?code=` / `http://127.0.0.1:<port>/callback?code=` |
| POST | `/api/auth/email/start` | `{email}` → always 202 (no account enumeration). Rate-limited, captcha optional (P07) |
| GET | `/api/auth/email/verify?token=` | Single-use, 15 min. Same client handling as the OAuth callback |
| POST | `/api/auth/token` | `{grant_type:"authorization_code", code, code_verifier}` or `{grant_type:"refresh_token", refresh_token}` → `{access_token, expires_in, refresh_token}` |
| POST | `/api/auth/logout` | Revokes the refresh token family |
| GET | `/api/me` | Profile, linked identities, limits |
| DELETE | `/api/me` | Deletes the account and all data (needs a confirmation token from `/api/me/delete-intent`) |

## Devices

| Method | Path | Body / notes |
|---|---|---|
| GET | `/api/devices` | List |
| POST | `/api/devices` | `{name, model, ios_version, client_id}` → device. `client_id` = the anisette v3 client identifier (opaque) |
| PATCH | `/api/devices/{id}` | `{name}` |
| DELETE | `/api/devices/{id}` | Removes the device and its history |
| POST | `/api/devices/{id}/report` | Status report from the app (below) |

**Report payload** (sent after every refresh attempt and on launch):
```json
{
  "reported_at": "2026-11-26T20:00:00Z",
  "vpn_ok": true,
  "pairing_ok": true,
  "apple_account": { "team_id": "ABCDE12345", "label": "j***@icloud.com", "app_ids_used_7d": 4 },
  "apps": [
    { "bundle_id": "com.example.app", "name": "Example", "version": "1.2.0",
      "expires_at": "2026-12-03T20:00:00Z", "source_url": "https://…/source.json" }
  ],
  "event": { "kind": "refresh", "result": "success|failure", "trigger": "background|shortcut|manual|launch",
             "error_code": null, "duration_ms": 8123 }
}
```
The server **must reject** any field it doesn't know (`DisallowUnknownFields`) so credentials can't be added by accident. `label` is masked on the device before sending.

## Apps, events, sources

| Method | Path | Notes |
|---|---|---|
| GET | `/api/devices/{id}/apps` | Latest inventory |
| GET | `/api/events?device_id=&result=&cursor=` | Paginated, newest first |
| GET/POST/DELETE | `/api/sources` | Saved AltStore-format source URLs per user. POST validates by fetching (timeout 5s, max 2 MB, https only, SSRF-safe resolver) |

## Web push

| Method | Path | Notes |
|---|---|---|
| GET | `/api/push/vapid-public-key` | |
| POST | `/api/push/subscriptions` | Browser `PushSubscription` JSON |
| DELETE | `/api/push/subscriptions/{id}` | |
| POST | `/api/push/test` | Sends a test notification to the caller's subscriptions |

## Anisette gateway

Proxies to the internal `anisette-v3:6969`. Requires a bearer token; the WebSocket accepts the token in the `Authorization` header (preferred) or `Sec-WebSocket-Protocol: orbit.bearer.<token>`.

| Method | Path → upstream |
|---|---|
| GET | `/anisette/v3/client_info` → `/v3/client_info` |
| POST | `/anisette/v3/get_headers` → `/v3/get_headers` |
| WS | `/anisette/v3/provisioning_session` → `/v3/provisioning_session` |

Limits (defaults, configurable): 60 `get_headers`/min per user, 5 provisioning sessions/hour per user, 3 per hour per device. Logs contain user ID, device ID, endpoint, status and latency. **Never** request or response bodies.

## Health

`GET /healthz` (process up) · `GET /readyz` (DB + anisette reachable) · `GET /metrics` (Prometheus; restrict to the internal network)
