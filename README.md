# Orbit

**Your own iOS app store with automatic refresh.** Multi-user, self-hosted, and free-Apple-ID friendly.

Orbit signs and installs apps **on your iPhone** and refreshes them before they expire, so you never need a computer after the one-time setup. Your Apple ID never leaves your device.

> Status: **planning kit / pre-alpha.** This repo has the specs, design system and agent instructions. Code is built phase by phase with Claude Code (see below).

## Pieces

| | |
|---|---|
| 📱 **Orbit (iOS)** | SwiftUI app: sources, install, auto-refresh, alerts |
| 🖥️ **Orbit Setup** | macOS / Windows / Linux one-time installer (fork of iloader). Drivers handled automatically |
| 🌐 **Dashboard** | Web app (PWA) showing every device, app and expiry, with web push alerts |
| ⚙️ **Server** | Go API + authenticated gateway to a self-hosted [anisette-v3-server](https://github.com/Dadoum/anisette-v3-server) |

## Build it with Claude Code

1. **Create an empty public GitHub repo** (no README/license), then unzip this kit into it and push:
   ```bash
   git init && git add . && git commit -m "chore: planning kit"
   git branch -M main && git remote add origin https://github.com/<you>/orbit.git && git push -u origin main
   ```
2. **Open the repo in Claude Code** (terminal: `claude` in the repo folder, or Claude Code on the web with the repo attached). Claude reads `CLAUDE.md` → `AGENTS.md` automatically.
3. **Run the phases in order** with the slash commands:

   | Command | Builds | You test |
   |---|---|---|
   | `/phase-0` | License, skeletons, design system, CI | CI is green |
   | `/phase-1` | Server, auth, anisette gateway, push, Docker deploy | `docker compose up` on your VPS |
   | `/phase-2` | Web dashboard PWA | Log in, enable push on iPhone |
   | `/phase-3` | iOS app | Sideload CI's `Orbit.ipa`, try every flow |
   | `/phase-4` | Orbit Setup desktop app | Install on Mac/Win/Linux with a phone |
   | `/phase-5` | LiveContainer variant + Shortcuts | 3 user apps on a free account |
   | `/phase-6` | Security audit, release pipeline | Approve v1.0.0 |

   Extras: `/review-ui`, and the subagents `ui-reviewer` and `security-reviewer` (in `.claude/agents/`).
4. **Secrets you'll need** (Phase 1): GitHub + Google OAuth apps, SMTP, a domain. See `deploy/.env.example` and `docs/SELF_HOSTING.md`. Put them in `deploy/.env` (git-ignored) and in GitHub Actions secrets, **never in chat or commits.**

No Mac? That's fine: iOS and macOS builds run in GitHub Actions.

## Docs
[Architecture](docs/ARCHITECTURE.md) · [Decisions](docs/DECISIONS.md) · [Roadmap](docs/ROADMAP.md) · [API](docs/API.md) · [Data model](docs/DATA_MODEL.md) · [Security](docs/SECURITY.md) · [iOS app](docs/IOS_APP.md) · [Desktop setup app](docs/DESKTOP_SETUP_APP.md) · [Dashboard](docs/WEB_DASHBOARD.md) · [Self-hosting](docs/SELF_HOSTING.md) · [User guide](docs/USER_GUIDE.md) · [Licensing](docs/LICENSING.md) · [UX checklist](docs/UX_CHECKLIST.md) · [Design system](design-system/orbit/MASTER.md)

## Renaming "Orbit"
"Orbit" and `dev.orbitstore` are placeholders. To rename before Phase 0, replace all occurrences:
```bash
grep -rl --exclude-dir=.git -e 'Orbit' -e 'orbitstore' -e 'orbit' . | xargs sed -i 's/orbitstore/yourprefix/g; s/Orbit/YourName/g; s/orbit/yourname/g'
```
Then review the diff, since generic words may also match.

## Credits & license
AGPL-3.0 (see `LICENSE`, added in Phase 0). Built on the work of [SideStore](https://github.com/SideStore/SideStore), [iloader](https://github.com/nab138/iloader), [anisette-v3-server](https://github.com/Dadoum/anisette-v3-server), [LiveContainer](https://github.com/LiveContainer/LiveContainer) and [UI-UX-Pro-Max](https://github.com/nextlevelbuilder/ui-ux-pro-max-skill). See `docs/LICENSING.md`.
