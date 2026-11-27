# Roadmap

Each phase is one PR (or a few sub-PRs) with a slash command in `.claude/commands/phase-N.md`. Tick boxes as you go. **Status:** ☐ todo · ◐ in progress · ☑ done

---

## Phase 0: Foundations ☑  → `/phase-0`
- [x] Repo scaffolding per AGENTS.md §2; add the `LICENSE` (AGPL-3.0 full text) and `NOTICE`
- [x] `.gitignore`, `.editorconfig`, PR template, CODEOWNERS
- [x] Design system in place (`design-system/orbit/*`); copy `OrbitTheme.swift` into the iOS project once it exists
- [x] CI skeletons that pass on an empty project: `server.yml`, `web.yml`, `ios.yml` (macos-latest, unsigned build), `desktop.yml` (matrix: macos/windows/ubuntu)
- [x] Branch protection on `main`: require CI + a pull request (0 approvals while there is one maintainer, D23)

**Done when:** all 4 workflows are green on `main`.

## Phase 1: Backend + deployment ◐  → `/phase-1`
- [x] `server/`: config (env), slog, health (`/healthz`, `/readyz`), Postgres + goose migrations from `DATA_MODEL.md`
- [x] Auth: GitHub + Google OAuth (PKCE, `state`), email magic link (SMTP, 15-min single-use hashed token), JWT access + rotating refresh tokens, iOS one-time-code exchange (`orbit://auth/callback`)
- [x] Devices, apps, refresh events, sources CRUD per `API.md`
- [x] **Anisette gateway**: proxy `/anisette/v3/client_info`, `/anisette/v3/get_headers`, `/anisette/v3/provisioning_session` (WebSocket) to the internal anisette container; bearer auth; per-user + per-IP rate limits; metadata-only logs
- [x] Web push: VAPID keys, subscription storage, a sender with expiry/backoff, test endpoint
- [x] Scheduler: expiry thresholds (24h / 6h), device-silent (48h), deduplicated via `alerts_sent`; prune expired login codes, refresh tokens and old events
- [x] `deploy/`: compose with caddy + server + postgres + anisette; `.env.example`; backup script
- [x] Tests: handlers (table-driven), auth flows (httptest + fake OAuth provider), gateway (WebSocket proxy test)
- [x] Resolve **P07** (abuse controls): D24

**Done when:** `docker compose up` on a fresh VPS gives HTTPS, login works with GitHub/Google/email, `curl` with a token reaches anisette through the gateway, and without a token gets 401.

*Checked on a local stack with the real anisette image (gateway with and without a token, backup). Still to check on a real VPS: HTTPS certificate and sign-in with real GitHub/Google/SMTP credentials. Commands: `docs/SELF_HOSTING.md` → Verify your install.*

## Phase 2: Web dashboard ◐  → `/phase-2`
- [x] SvelteKit + adapter-static, embedded into the Go binary (`embed.FS`)
- [x] Pages per `WEB_DASHBOARD.md` + `design-system/orbit/pages/dashboard.md`
- [x] Self-host IBM Plex Sans + JetBrains Mono (`font-display: swap`, OFL license file); the Phase 0 skeleton uses the system fallback
- [x] PWA manifest, service worker, web push subscribe/unsubscribe/test, iOS "Add to Home Screen" guidance
- [x] Playwright: login (mocked), device list, enabling push; axe accessibility check with no serious violations
- [x] UX checklist passed; light + dark screenshots at 375 / 768 / 1440

**Done when:** a user can log in, see a seeded device and apps, and receive a test web push on an iPhone home-screen PWA.

*Checked in headless Chromium against the real stack: sign-in, session renewal, service worker, sign-out. Still to check by a human: a real push arriving on an iPhone Home Screen app (needs a deployed server with HTTPS).*

## Phase 3: iOS app ◐  → `/phase-3`
- [ ] Resolve **P01** first (SideStore library choice); record in DECISIONS.md. *Researched; the dependency choice awaits the owner (see DECISIONS → P01 research)*
- [x] Xcode project via **XcodeGen** (`ios/project.yml`) so it builds on CI without a Mac; SPM deps
- [x] Orbit login (`ASWebAuthenticationSession`), Keychain token storage. *Not yet tried on a device*
- [ ] Apple ID sign-in (anisette via the gateway, 2FA), Keychain storage, certificate management (create/reuse/revoke with a confirmation)
- [ ] Pairing file import/detect; VPN reachability check; **re-pair flow** (detect invalid → notify → guide)
- [ ] App sources (AltStore source JSON), app detail, install, refresh, remove; App ID usage tracker (10/week). *Done: sources, app detail, add and remove a source. Waiting on P01: install, refresh, remove an app, App ID tracker*
- [ ] Refresh engine: BGAppRefreshTask + App Intent "Refresh Apps"; local notifications (T-48/24/6h + on failure). *Done: reminders for Orbit's own expiry. The rest needs P01*
- [ ] Status reporting to the server (`POST /api/devices/:id/report`). *Done: registration and check-in. App inventory and events need P01*
- [ ] UI per `design-system/orbit/pages/ios.md`; Dynamic Type AX5; VoiceOver labels
- [x] CI: unsigned `Orbit.ipa` artifact

**Done when:** on a real iPhone (installed via iloader for now), the user can sign in, add a source, install an app, and see it refresh through the Shortcut and through background refresh, with matching events on the dashboard.

## Phase 4: Orbit Setup desktop app ☐  → `/phase-4`
- [ ] Fork iloader into `desktop/` (git subtree, keep upstream remote), rebrand, keep the MIT notice
- [ ] Orbit flow: Orbit login → device detect → drivers → Apple ID → install Orbit → pairing file → phone checklist
- [ ] Windows: detect AMDS / Apple Devices; silent install via winget (resolve **P03**) with progress; fallback
- [ ] Linux: bundled usbmuxd + udev rule via pkexec on first run (resolve **P04** for Flatpak)
- [ ] Packaging: dmg (universal), msi/nsis, AppImage, deb, rpm, Flatpak manifest, AUR `PKGBUILD` (`orbit-setup-bin`)
- [ ] UI per `design-system/orbit/pages/desktop.md`

**Done when:** on clean Windows 11, Ubuntu 24.04, Fedora, Arch and macOS machines, a non-technical tester gets Orbit installed and paired without installing anything by hand.

## Phase 5: LiveContainer + Shortcuts ☐  → `/phase-5`
- [ ] Resolve **P05**; build the "LiveContainer + Orbit" variant (CI artifact), selectable in Orbit Setup
- [ ] Guided Shortcuts setup: open the prebuilt shortcut via an iCloud link, or step-by-step instructions; resolve **P02** (VPN toggle)
- [ ] Slots meter + "free up a slot" guidance

**Done when:** a free account runs Orbit + 3 user apps on one device using LiveContainer, and the daily automation refreshes them with the VPN handled.

## Phase 6: Hardening & release ◐  → `/phase-6`
- [ ] `security-reviewer` subagent full audit; fix findings; update SECURITY.md. *Done for what exists; P08-P10 and P12 await decisions, and the audit repeats when iOS signing and the desktop app exist*
- [x] Load test the anisette gateway (k6); tune rate limits (results in API.md; limits unchanged)
- [x] Observability: structured logs, `/metrics` (Prometheus), uptime check
- [ ] Docs: USER_GUIDE with screenshots, SELF_HOSTING, troubleshooting. *Written; screenshots are placeholders*
- [ ] `release.yml`: tag → build all artifacts → GitHub Release with checksums; resolve **P06**. *Server image and IPA only; desktop packages and the LiveContainer variant don't exist yet; P06 pending*
- [ ] v1.0.0

**Done when:** a tagged release publishes every artifact, and the docs let a stranger self-host and onboard.
