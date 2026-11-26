# AGENTS.md: Orbit

Orbit is a multi-user iOS sideloading service. It signs and installs apps **on the device** (SideStore-style) and refreshes them automatically. It has a Go backend, a SvelteKit dashboard, a self-hosted `anisette-v3-server`, and a cross-platform "Orbit Setup" desktop app for the one-time install.

This file is the canonical brief for **any** coding agent (Claude Code reads it through `CLAUDE.md`). If you change a decision, update `docs/DECISIONS.md` in the same PR.

---

## 1. Hard rules (never break these)

1. **Apple ID credentials never leave the user's device.** No Apple password, 2FA code, session token, certificate private key or `.p12` may be sent to, logged by or stored on the Orbit server. Apple auth traffic goes device → Apple directly. The server only sees anisette requests (which contain no credentials) and app-status reports.
2. **Never commit secrets.** `.env`, OAuth secrets, VAPID private keys, SMTP passwords, `*.p12`, `*.mobileprovision`, pairing files (`*.mobiledevicepairing`, `*.plist` pairing records) and `*.pem` are git-ignored. Only use `*.example` files.
3. **Licensing:** the repo is **AGPL-3.0** because we link SideStore code. Keep license headers. Credit SideStore, iloader (MIT), anisette-v3-server and LiveContainer in About screens and in `docs/LICENSING.md`. Check a dependency's license before adding it.
4. **Free-account reality:** the app is installed with *users' free Apple IDs*. Never rely on entitlements a free provisioning profile can't grant: **no APNs push, no Sign in with Apple, no Network Extension, no Associated Domains, no App Groups across teams, no iCloud.** Allowed: URL schemes, `ASWebAuthenticationSession`, local notifications, BackgroundTasks, App Intents, Keychain (own access group).
5. **Design system is mandatory for UI.** Read `design-system/orbit/MASTER.md` plus the relevant `pages/*.md` before writing any UI. Use tokens only, never raw hex values. Run the pre-delivery checklist in `docs/UX_CHECKLIST.md` before marking UI work done.
6. **Don't break user state.** Any change to pairing files, certificates or installed apps must be reversible or must ask first. Destructive actions need a confirmation that names the thing being removed.

## 2. Repo map

```
AGENTS.md / CLAUDE.md     agent instructions (this file is canonical)
docs/                     architecture, decisions, roadmap, API, data model, security, per-component specs
design-system/orbit/      MASTER.md, tokens.{json,css}, OrbitTheme.swift, pages/*.md
server/                   Go 1.23+ API (chi, pgx, sqlc, goose), embeds web/build
web/                      SvelteKit 2 + Svelte 5 (runes), adapter-static, PWA + web push
ios/                      SwiftUI (iOS 16+ target, iOS 17+ APIs behind availability), SPM
desktop/                  Orbit Setup: fork of nab138/iloader (Tauri 2 + Rust + TS)
deploy/                   docker-compose (caddy, orbit-server, postgres, anisette-v3)
.github/workflows/        server.yml, web.yml, ios.yml, desktop.yml, release.yml
.claude/                  commands/ (phase prompts), agents/ (reviewers), settings.json
```

## 3. Tech stack & conventions

| Area | Choice | Conventions |
|---|---|---|
| Backend | Go, `chi` router, `pgx/v5`, `sqlc`, `goose` migrations, `slog` | Packages are `internal/{auth,devices,apps,events,anisette,push,mail,store}`. Errors are wrapped with `%w`. Every handler has a table-driven test. `go vet` + `staticcheck` stay clean |
| DB | PostgreSQL 16 | UUIDv7 primary keys, `timestamptz`, soft-delete only for users, migrations are append-only |
| Auth | GitHub + Google OAuth2 (PKCE), email magic link | Access JWT lasts 15 min. Refresh tokens are opaque, hashed (SHA-256) and rotated on use, with reuse detection |
| Web | SvelteKit (adapter-static), TypeScript strict, Lucide icons | Use `$state`/`$derived`/`$props` (runes) and `tokens.css` only. Vitest + Playwright |
| iOS | SwiftUI, `@Observable` view models (iOS 17) with an `ObservableObject` fallback where needed, MVVM, SPM | Keep views small. Use `OrbitTheme` tokens. Run async work in `.task`, with `@MainActor` view models. XCTest for view models |
| Desktop | Tauri 2, Rust (`idevice` crate as used by iloader), TS frontend | Keep the upstream iloader structure so it's easy to rebase. Orbit-specific code goes in `desktop/src-orbit/` / clearly marked modules |
| Infra | Docker Compose, Caddy (auto TLS) | One `.env`. Images pinned by digest in releases |

Commits use Conventional Commits (`feat(server): …`, `fix(ios): …`). One PR per roadmap phase or sub-phase, with a summary, a test plan and screenshots for UI.

## 4. Commands

```bash
# server (database tests need TEST_DATABASE_URL pointing at a Postgres 16; they skip without it)
cd server && go test ./... && go vet ./... && staticcheck ./...
cd server && sqlc generate   # after editing internal/store/queries or migrations; commit the result
cd server && goose -dir migrations postgres "$DATABASE_URL" up
# web
cd web && pnpm i && pnpm check && pnpm test && pnpm build
# deploy (local)
cd deploy && cp .env.example .env && docker compose up -d --build
# ios (macOS / CI only)
xcodebuild -project ios/Orbit.xcodeproj -scheme Orbit -configuration Release \
  -destination 'generic/platform=iOS' CODE_SIGNING_ALLOWED=NO build
# desktop
cd desktop && bun install && bun tauri build
```

The development machine has **no Mac**. iOS and macOS desktop builds run only in GitHub Actions (`macos-latest`). Don't claim an iOS change works until `ios.yml` passes, and say clearly what still needs testing on a device.

## 5. Where to look first

- What we're building and why: `docs/ARCHITECTURE.md`, `docs/DECISIONS.md`
- What to do next: `docs/ROADMAP.md` (phase checklist and acceptance criteria)
- API contract: `docs/API.md` · Schema: `docs/DATA_MODEL.md` · Threats: `docs/SECURITY.md`
- Component specs: `docs/IOS_APP.md`, `docs/DESKTOP_SETUP_APP.md`, `docs/WEB_DASHBOARD.md`
- Ops: `docs/SELF_HOSTING.md` · End users: `docs/USER_GUIDE.md` · Licenses: `docs/LICENSING.md`

## 6. Open questions (verify before you build on them, then record the answer in DECISIONS.md)

- Which SideStore libraries are current: `minimuxer` vs `idevice`, and whether `em_proxy` is still needed with LocalDevVPN. Check SideStore's `develop` branch at the start of Phase 3.
- LocalDevVPN's URL scheme and App Intents (whether Shortcuts can toggle it).
- Exact winget / package IDs for Apple Devices / Apple Mobile Device Support on Windows.
- A Linux USB access strategy that works inside a Flatpak sandbox.
- LiveContainer licensing and its "built-in store" integration path.
