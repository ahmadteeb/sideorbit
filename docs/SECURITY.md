# Security & threat model

## Assets (most → least sensitive)
1. Users' **Apple ID credentials, sessions, dev certificates/private keys, pairing records.** These are on the device only.
2. Orbit refresh tokens / sessions.
3. The anisette service (an abuse target: free Apple provisioning for spammers).
4. Device and app inventory (privacy: which apps someone sideloads).
5. Operator secrets: OAuth client secrets, VAPID private key, SMTP credentials, DB password.

## Trust boundaries & controls

| Threat | Control |
|---|---|
| Server breach exposes Apple accounts | **The server never receives them** (hard rule 1). The report endpoint rejects unknown fields. A CI grep test fails the build if the server code references `password`, `p12` or `pairing` outside allowlisted files |
| Pairing file theft on device | Stored in the app container with `FileProtectionType.complete`. Never uploaded. Exports require an explicit user action |
| Apple credentials at rest on device | Keychain with `kSecAttrAccessibleAfterFirstUnlockThisDeviceOnly` (so background refresh still works). Never in UserDefaults or logs |
| Anisette abuse | Gateway auth required, per-user/device/IP rate limits, provisioning caps, account device cap (P07), anomaly logging. The container is not exposed publicly |
| Account takeover via OAuth | PKCE + `state` + exact redirect allowlist. The iOS code exchange is bound to a `code_challenge`. Codes are single-use with a 60s TTL. Native sign-ins need a button press first (D32). **Residual risk:** `orbit://` can be claimed by another app; PKCE and the button limit but don't remove it |
| Magic link attacks | 15-min single-use hashed tokens, rate limits per email + IP, no enumeration (always 202) |
| Refresh token theft | Rotation with reuse detection (revoke family), 30-day absolute expiry, device-bound `client` field |
| SSRF via source URLs | https only, resolve and block private/loopback/link-local ranges, 5s timeout, 2 MB cap, no redirects to blocked ranges |
| Malicious IPAs from sources | Orbit shows the source URL, bundle ID and requested entitlements before install. Warn on unknown sources. We don't host IPAs |
| XSS / CSRF on the dashboard | Strict CSP (no inline scripts), SameSite=Lax cookies + CSRF token for cookie-auth mutations, output escaping (Svelte default) |
| Desktop helper privilege | The polkit/UAC prompt only performs a fixed, auditable action (udev rule install / winget Apple Devices). No arbitrary command execution |
| Supply chain | Pin dependencies; Dependabot; actions pinned by SHA; release checksums; reproducible Docker builds |

## Logging rules
- Never log: request bodies on `/anisette/*`, `Authorization` headers, tokens, emails in full (mask them), IPs beyond 30 days.
- Always log: auth events (success/failure), rate-limit hits, admin actions.

## Reporting
A `SECURITY.md` at the repo root (copy of this section's contact) tells people to report privately via GitHub Security Advisories.
