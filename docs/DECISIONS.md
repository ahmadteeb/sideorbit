# Decision log (ADR-lite)

Format: **ID · Decision** · *Why* · Alternatives rejected. Add new entries at the bottom and never rewrite old ones. To reverse a decision, add a new one that supersedes it.

| ID | Decision | Why | Rejected |
|---|---|---|---|
| D01 | **On-device signing & install (SideStore-style)** | Enables true background refresh with no computer after setup | Central server + OTA (manual tap per refresh); desktop companion (needs a computer always on) |
| D02 | **Open sign-up multi-user service** | Product goal | Single-user / family-only |
| D03 | **Free Apple IDs are the primary target** | Most users don't pay $99/yr. Auto-refresh matters most here | Paid-only |
| D04 | **Self-host anisette-v3-server in Docker behind our gateway** | Per-client provisioning makes multi-user safe. The gateway adds auth and quotas | Public anisette servers (reliability, privacy); per-user anisette containers (unneeded with v3) |
| D05 | **No Mac: build iOS and macOS in GitHub Actions** | Owner has no Mac | Local Xcode |
| D06 | **Apple credentials stay on the device only (Keychain)** | Security and legal liability of storing strangers' Apple IDs | Encrypted server-side storage |
| D07 | **Backend in Go** | Single static binary, small image, good WebSocket reverse proxy | Node, Python, Rust |
| D08 | **v1 scope: background refresh, app sources, dashboard, alerts** | Owner choice | — |
| D09 | **Reuse SideStore libraries → repo is AGPL-3.0 and public** | Proven protocol code; months saved | From scratch; full SideStore fork |
| D10 | **Login: GitHub + Google OAuth + email magic link** | Works in free-signed apps via `ASWebAuthenticationSession`, no $99 needed | Sign in with Apple (needs a paid account + entitlement), passkeys (need Associated Domains) |
| D11 | **Alerts: local notifications + web push (PWA)** | APNs is impossible in a free-signed app. Web push needs no Apple account | APNs; email (maybe later) |
| D12 | **One-time setup via our own desktop app forked from iloader (MIT)** | Best UX; legal to fork | Pointing users to iloader; on-device pairing first (newer, less proven; reconsider for v2) |
| D13 | **LiveContainer bundle, VPN status + 1-tap, Shortcuts automation, auto re-pair** | Saves app slots; makes refresh reliable; makes failures recoverable | — |
| D14 | **Desktop app on macOS, Windows, Linux with no manual driver installs** | Owner requirement | — |
| D15 | **Windows: auto-install Apple's driver (Apple Devices) silently via winget** | Most compatible; Apple's driver can't be redistributed | Own WinUSB driver (breaks iTunes / Apple Devices) |
| D16 | **Linux: bundled usbmuxd + one-time polkit prompt for udev rules** | No separate install for the user | Asking users to install usbmuxd |
| D17 | **Desktop = iloader fork (Tauri 2 + Rust)** | Already cross-platform and working | Fresh Tauri app; Go + Wails |
| D18 | **Packages: dmg (universal), Windows installer, AppImage, .deb, .rpm, Flatpak, Arch (AUR)** | Owner choice | — |
| D19 | **Dashboard in SvelteKit (adapter-static) embedded in the Go binary** | One container; fast; small | Next.js |
| D20 | **Design: UI-UX-Pro-Max data → Dark OLED + Swiss minimal, green = fresh, system theme** | Matches "Developer Tool" reasoning; calm status-first UX | — |
| D21 | **Product name "Orbit" (placeholder), bundle prefix `dev.orbitstore`** | Easy to rename (see README "Renaming") | — |
| D22 | **iOS minimum: 16.0** (iOS 17 APIs behind `#available`) | Developer Mode exists from 16. Covers most devices that LocalDevVPN supports | 15 |

## Pending decisions (to be resolved during the named phase)

| ID | Question | Phase |
|---|---|---|
| P01 | `minimuxer` vs `idevice` on iOS; is `em_proxy` still required with LocalDevVPN? | 3 |
| P02 | LocalDevVPN URL scheme / App Intents for Shortcuts | 3 / 5 |
| P03 | winget IDs for Apple Devices / AMDS; fallback when winget is missing (older Windows 10) | 4 |
| P04 | Flatpak USB strategy (host usbmuxd via portal vs `--device=all` + bundled daemon) | 4 |
| P05 | LiveContainer license and integration (built-in store mode) | 5 |
| P06 | Code signing: Apple notarization / Windows Authenticode certificates (cost vs warnings) | 6 |
| P07 | Abuse controls for open sign-up: captcha (Turnstile) on email link? device cap per account? | 1 |
