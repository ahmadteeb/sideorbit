# API contract (v1 draft)

Base URL: `https://<ORBIT_DOMAIN>`. JSON everywhere. Auth uses `Authorization: Bearer <access_jwt>`. Errors use `{"error": {"code": "string", "message": "human text", "fix": "optional human hint"}}`. Times are RFC 3339 UTC. IDs are UUIDv7 strings.

## Auth

| Method | Path | Notes |
|---|---|---|
| GET | `/api/auth/{github\|google}/start?client=web\|ios\|desktop&code_challenge=&redirect=` | Sets the `state` cookie and redirects to the provider |
| GET | `/api/auth/{provider}/callback` | Web: sets the session and redirects to `/`. ios/desktop: shows a "Continue as …" page (D32). Failures redirect with `?error=access_denied\|email_unverified\|account_exists\|provider_error` (web: to `/login`). `account_exists`: sign in the way you did before, or by email link (D25) |
| POST | `/api/auth/email/start` | `{email, client?, code_challenge?, redirect?}` → 202 whether or not an account exists. 3 per address and 10 per IP per 15 min, 100 per hour in total (429); 503 if mail can't be sent. No captcha (D24) |
| GET | `/api/auth/email/verify?token=` | A confirmation page. Does not use up the token, so mail scanners can't burn it |
| POST | `/api/auth/email/verify` | Form field `token`. Single-use, 15 min, same-origin only. Same client handling as the OAuth callback |
| GET | `/api/auth/methods` | `{github, google, email}`: which sign-in methods are configured. Public |
| POST | `/api/auth/confirm` | The native page's button (form field `token`, 5 min, same-origin only). Redirects to `orbit://auth/callback?code=` / `http://127.0.0.1:<port>/callback?code=` |
| POST | `/api/auth/token` | `{grant_type:"authorization_code", code, code_verifier}` or `{grant_type:"refresh_token", refresh_token}` → `{access_token, expires_in, refresh_token}` |
| POST | `/api/auth/logout` | `{refresh_token}` (or the web cookie). Revokes the refresh token family; 204 |
| GET | `/api/me` | Profile, linked identities, limits |
| POST | `/api/me/delete-intent` | `{confirmation_token, expires_in}` (5 min) |
| GET | `/api/me/sessions` | `{sessions:[{id, client, created_at, last_used_at, current}]}` |
| DELETE | `/api/me/sessions/{id}` | Signs that session out (its access token lasts up to 15 min more) |
| DELETE | `/api/me` | `{confirmation_token}`. Deletes the account and all data (D27) |

### Sessions

- `client` defaults to `web`. `ios` and `desktop` need a PKCE S256 `code_challenge`. `ios` defaults to `orbit://auth/callback`; `desktop` must send its loopback `redirect` (any port).
- JSON bodies must be sent as `Content-Type: application/json` (415 otherwise).
- Access tokens last 15 minutes and stay valid until then, even after logout. Refresh tokens rotate on every use and expire 30 days after sign-in. **Using an old one revokes the whole session**, so a client must never refresh twice in parallel.
- **Web** keeps both tokens in HttpOnly cookies (`__Host-orbit_access`, `__Host-orbit_refresh`; no prefix on http://localhost). Every cookie-authenticated mutation, including refresh and logout, must send an `X-Orbit-CSRF` header (any value). Refresh with `{"grant_type":"refresh_token"}`; the response carries only `expires_in`.

## Devices

| Method | Path | Body / notes |
|---|---|---|
| GET | `/api/devices` | `{devices:[…]}` |
| POST | `/api/devices` | `{name, model, ios_version, client_id}` → 201 device. `client_id` = the anisette v3 client identifier (opaque). The same `client_id` again → 200 with the existing device. 403 `device_limit` at the cap, 409 `client_id_in_use` if another account has it |
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

→ 204. Only `reported_at` is required (within the last 90 days; at most 10 minutes ahead). Limits: 60 reports/min per user, 100 apps.
- `apps` present replaces the inventory (an empty list clears it); absent leaves it unchanged.
- A report older than the newest one already received only adds its event to history. Resending a report adds nothing.
- `label` must be masked (at least `***`, at most 4 visible characters before `@`). `error_code` is `UPPER_SNAKE_CASE`, never free text. `source_url` is https without credentials. `trigger` may also be `setup`.

## Apps, events, sources

| Method | Path | Notes |
|---|---|---|
| GET | `/api/devices/{id}/apps` | Latest inventory |
| GET | `/api/events?device_id=&result=&cursor=` | `{events:[…], next_cursor}`, newest first, 50 per page |
| GET | `/api/sources` | `{sources:[{id,url,name,created_at}]}` |
| POST | `/api/sources` | `{url}`. Validates by fetching (timeout 5s, max 2 MB, https only, SSRF-safe resolver). 10/min and 50 saved per user |
| POST | `/api/sources/preview` | `{url}` → `{url, name, app_count, apps}` (first 5 names). Saves nothing; shares the add limit |
| DELETE | `/api/sources/{id}` | 204 |
| GET | `/api/events/summary` | `{days:[{date, success, failure}]}`: refreshes per UTC day, last 30 days |

## Web push

| Method | Path | Notes |
|---|---|---|
| GET | `/api/push/vapid-public-key` | `{public_key}`. The push routes exist only when VAPID keys are configured |
| POST | `/api/push/subscriptions` | Browser `PushSubscription` JSON → `{id}`. Same endpoint → same id (re-posting moves it to the caller). At most 10 per account. Signing that web session out removes it |
| DELETE | `/api/push/subscriptions/{id}` | |
| POST | `/api/push/test` | `{sent, failed}`; 409 with no subscriptions; 5/min |

Notification payload: `{title, body, url}` (`url` is a dashboard path).

Alerts (every 15 min): one per device when apps expire within 24 h, another within 6 h, and one when a device with apps hasn't reported for 48 h. A refresh re-arms them. Delivery failures are retried; subscriptions reported gone, or failing 10 times, are removed.

## Anisette gateway

Proxies to the internal `anisette-v3:6969`. Requires a bearer token; the WebSocket accepts the token in the `Authorization` header (preferred) or `Sec-WebSocket-Protocol: orbit.bearer.<token>`.

| Method | Path → upstream |
|---|---|
| GET | `/anisette/v3/client_info` → `/v3/client_info` |
| POST | `/anisette/v3/get_headers` → `/v3/get_headers` |
| WS | `/anisette/v3/provisioning_session` → `/v3/provisioning_session` |

- Only a bearer token is accepted (never cookies or query strings). A WebSocket client that offers `orbit.bearer.<token>` gets that subprotocol echoed back.
- Send `X-Orbit-Device: <device id>` once the device is registered; an unknown or foreign id → 403 `unknown_device`.
- The upstream receives only content and WebSocket headers. A provisioning session is closed after 2 minutes; `get_headers` bodies are at most 64 KB.
- 502 `anisette_unavailable` if the upstream is down.

Limits (defaults, configurable): 60 requests/min per user across `client_info` and `get_headers`, 5 provisioning sessions/hour per user, 3 per hour per device, 120 requests/min per IP. Provisioning is also capped at 10/hour per IP and 100/hour in total, so that recreating accounts doesn't reset the quota. The per-device limit applies only when `X-Orbit-Device` is sent (a first sign-in may not have a device yet). 429 carries `Retry-After`. Logs contain user ID, device ID, endpoint, status and latency. **Never** request or response bodies.

## Health

`GET /healthz` (process up) · `GET /readyz` (`{db, anisette}`, 503 if either is down; result reused for 5 s) · `GET /metrics` arrives in Phase 6 (Caddy already hides it)
