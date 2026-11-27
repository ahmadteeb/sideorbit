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
| Pairing file theft on device | *Not built yet.* Stored in the app container, never uploaded, exports need an explicit action. The file protection class is open (P11): `complete` would make it unreadable during a locked-screen refresh |
| Apple credentials at rest on device | Keychain with `kSecAttrAccessibleAfterFirstUnlockThisDeviceOnly` (so background refresh still works). Never in UserDefaults or logs |
| Anisette abuse | Gateway auth required, per-user/device/IP/total rate limits, provisioning caps, account device cap (D24), rate-limit hits logged and counted in `/metrics`. The container is not exposed publicly |
| Account takeover via OAuth | PKCE + `state` + exact redirect allowlist. The iOS code exchange is bound to a `code_challenge`. Codes are single-use with a 60s TTL. Native sign-ins need a button press first (D32). **Residual risk:** `orbit://` can be claimed by another app; PKCE and the button limit but don't remove it |
| Account pre-claimed with a stale address | The first sign-in that proves the mailbox is asked about an account that never did; "No" removes its other sign-in methods, sessions and data (D38). **Relies on the person reading the page** |
| Magic link attacks | 15-min single-use hashed tokens, rate limits per email + IP, no enumeration (always 202) |
| Refresh token theft | Rotation with reuse detection (revoke family), 30-day absolute expiry. A web token works only from the cookie and a native token only from the request body (not bound to a device) |
| SSRF via source URLs | https only, resolve and block private/loopback/link-local ranges, 5s timeout, 2 MB cap, no redirects to blocked ranges |
| Malicious IPAs from sources | Orbit shows the source URL, bundle ID and requested entitlements before install. Warn on unknown sources. We don't host IPAs |
| XSS / CSRF on the dashboard | Strict CSP (no inline scripts), `__Host-` SameSite=Lax cookies + a required CSRF header on cookie-auth mutations, output escaping (Svelte default) |
| Secrets in desktop logs and files | Only Orbit Setup's own code logs at debug level; the proxy secret is removed from error text; without a keyring, files live in an owner-only folder; apps are unpacked in a private temp folder on Linux. Developer tools and the auto-updater are off |
| Local programs and the anisette proxy | Binds `127.0.0.1` only, needs a 256-bit path secret, forwards three endpoints, and drops the caller's credentials |
| Desktop helper privilege | The polkit/UAC prompt only performs a fixed, auditable action (udev rule install / winget Apple Devices). No arbitrary command execution |
| Supply chain | Lockfiles; monthly Dependabot with a 14-day cooldown; actions pinned by SHA; anisette image pinned by digest; releases only from commits on `main`, with checksums. *Not yet:* other base images pinned by digest, reproducible builds |

## Known open risks

Found in the 2026-11-27 audit and waiting for a decision (see DECISIONS.md → Pending):

- **P10:** the shared caps on sign-in email and on provisioning can be used up by someone with several addresses and accounts, blocking others for an hour.
- **P12:** saved source URLs are stored whole, including any secret query token, and backups are unencrypted and keep deleted data for 14 days.

## Audit record

| Date | Scope | Result |
|---|---|---|
| 2026-11-26/27 | Per-PR reviews of auth, the gateway and iOS sign-in | Fixed before merge |
| 2026-11-27 | Whole repository | No path for Apple credentials to reach the server. 2 high, 4 medium, 6 low. Fixed: slow push endpoints stalling alerts, quota drained by plain GETs, push subscriptions from dead sessions, unbounded report history, tokens and addresses in proxy logs, permission screen wording, deletion race, anisette capabilities, stricter report fields, restore and example-secret handling. Open then: P08-P10, P12. P08 and P09 closed by D38 and D39 |

A compromised anisette image could still reach the API on their shared network and forge the client address that rate limits use. The image is pinned by digest.

## Logging rules
- Never log: request bodies on `/anisette/*`, `Authorization` headers, tokens, emails in full (mask them), client IP addresses (neither the server nor the proxy's error log records them).
- Always log: auth events (success/failure), rate-limit hits, admin actions.

## Reporting
A `SECURITY.md` at the repo root (copy of this section's contact) tells people to report privately via GitHub Security Advisories.
