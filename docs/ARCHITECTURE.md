# Architecture

## 1. Components

```
                         ┌───────────────────────── Your VPS (docker compose) ─────────────────────────┐
                         │                                                                            │
 iPhone (Orbit app) ─────┼─► Caddy (TLS) ─┬─► orbit-server (Go) ──► PostgreSQL                         │
   │  ▲                  │                │      │  ├─ /api/*        auth, devices, apps, events       │
   │  │ local only       │                │      │  ├─ /anisette/*   authenticated gateway ──────────┐ │
   │  │                  │                │      │  └─ /*            embedded SvelteKit dashboard    │ │
   │  └ LocalDevVPN      │                │      └─ scheduler: expiry checks → web push / email    │ │
   │    (loopback)       │                └───────────────────────────────────────────────────────┐ │ │
   │                     │                         anisette-v3-server :6969 (internal only) ◄────┘◄┘ │
   │                     └────────────────────────────────────────────────────────────────────────────┘
   │
   └──────────────────────► Apple (gsa.apple.com, developerservices2.apple.com) DIRECTLY.
                            Credentials and certificates never pass through Orbit servers.

 Computer (once) ── Orbit Setup (Tauri) ── USB ──► iPhone: install Orbit + write pairing file
                         └── also uses the anisette gateway (with an Orbit login) for the first Apple sign-in
```

## 2. Why each piece exists

| Piece | Job | Notes |
|---|---|---|
| **Orbit iOS app** | Holds the Apple ID session (Keychain), the dev certificate and the pairing file. Re-signs and installs IPAs on the device via the loopback VPN + pairing record, and reports status to the server | Forked/linked from SideStore libraries (AGPL) |
| **LocalDevVPN** (D31) | Third-party App Store VPN that routes a loopback address so iOS thinks a trusted "computer" is connected | Must be ON during install/refresh. Orbit shows its status and deep-links to it |
| **Pairing file** | Lockdown pairing record that lets Orbit talk to `installd` / `lockdownd` as the trusted host | Created by Orbit Setup. Can be invalidated by iOS updates/resets, so Orbit has a re-pair flow |
| **anisette-v3-server** | Provides Apple "anisette" headers (device provisioning data) that Apple's auth endpoints require | Unmodified upstream Docker image, never exposed publicly. Only reachable through the gateway |
| **Anisette gateway** (in orbit-server) | Reverse-proxies `/v3/client_info`, `/v3/get_headers` and the `/v3/provisioning_session` WebSocket. Requires a valid Orbit access token, rate-limits per user/device and logs **metadata only** | Turns a public anisette server (abuse target) into a members-only one |
| **orbit-server** | Orbit accounts (OAuth / magic link), device registry, app inventory, refresh events, alert scheduling, web push, serves the dashboard | Stateless except for Postgres |
| **Dashboard (PWA)** | Lets users see their devices, apps, expiry dates and history. Receives web push when added to the home screen | Static build embedded in the Go binary |
| **Orbit Setup** | Cross-platform one-time installer: detects the device, gets drivers, signs and installs Orbit, generates and places the pairing file | Fork of iloader (MIT). Mac/Win/Linux |

## 3. Why multi-user works with anisette v3

Anisette v3 provisions **per client**. Each Orbit installation creates its own client identifier and ADI state (`adi.pb`), kept **on the device**. The server performs provisioning with Apple over the WebSocket session and computes headers for that client, so users don't share one machine identity the way they did with v1 anisette servers. That's why one self-hosted anisette server can safely serve many users and devices. The gateway adds what upstream lacks: authentication, quotas and abuse controls.

## 4. Key flows

### 4.1 First-time setup (computer, once per device)
1. The user downloads Orbit Setup and signs in with their Orbit account (browser OAuth). This gives the anisette gateway access.
2. The user plugs in the iPhone and taps Trust. Orbit Setup gets drivers if needed (Windows: Apple Devices via winget; Linux: bundled usbmuxd + one-time polkit prompt).
3. Orbit Setup signs in to the Apple ID **locally** (anisette via the gateway), creates or uses a dev certificate, signs Orbit.ipa (or LiveContainer+Orbit) and installs it.
4. Orbit Setup generates the pairing record and writes it into Orbit's documents container (the same mechanism iloader uses for SideStore).
5. On the phone: enable Developer Mode (iOS 16+), trust the profile, install LocalDevVPN, open Orbit, sign in to Orbit and the Apple ID again (Keychain on the device).

### 4.2 Automatic refresh (on device)
1. Triggers: BackgroundTasks (`BGAppRefreshTask`, best effort), the App Intent "Refresh Apps" run by a Shortcuts personal automation (reliable, daily), app launch, or a manual tap.
2. Preconditions: VPN reachable, pairing valid, Apple session valid (re-auth silently with Keychain credentials plus anisette; if 2FA is needed, notify the user).
3. For each app expiring within the window (default ≤ 2 days): fetch or renew the free provisioning profile, re-sign, install via the loopback connection, then verify.
4. Report `refresh_event` (+ new expiry) to the server. Reschedule local notifications.
5. On failure: local notification with cause and fix, and a server event. The server sends web push if the device stays silent.

### 4.3 Alerts
- **Local (device):** scheduled at T-48h, T-24h and T-6h before the earliest expiry, plus immediately on failure. These are rescheduled after every refresh.
- **Server (web push):** a scheduler runs every 15 min. For each app where `expires_at - now < 24h` and no successful refresh since `expires_at - 7d`, it sends one push per threshold (deduplicated in `alerts_sent`). It also alerts when a device hasn't checked in for more than 48h.

### 4.4 Login from the iOS app (no special entitlements needed)
`ASWebAuthenticationSession` → `https://<domain>/api/auth/<provider>/start?client=ios&code_challenge=…` → provider → server callback → redirect `orbit://auth/callback?code=<one-time>` → the app POSTs `/api/auth/token` with the `code_verifier` → access + refresh tokens (Keychain).

## 5. Free Apple ID constraints we design around

- Apps expire after **7 days**, hence the refresh window and the alerts.
- **3 active sideloaded apps per device**, including Orbit itself. LiveContainer bundling frees a slot.
- **10 App IDs per 7 days** per account (app extensions count too). Orbit tracks App ID usage and warns before it runs out.
- No entitlements beyond the basics (see AGENTS.md hard rule 4).

## 6. Deployment topology

A single VPS (2 vCPU / 2 GB is enough to start). Caddy terminates TLS for `orbit.example.com`. anisette-v3 is on an internal Docker network only. Postgres lives in a named volume with nightly `pg_dump` (see SELF_HOSTING.md). To scale out, run more orbit-server replicas (they're stateless) and anisette replicas behind the gateway. The gateway uses sticky routing by client ID for the provisioning WebSocket.
