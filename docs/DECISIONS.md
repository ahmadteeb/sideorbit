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
| D23 | **`main` requires a pull request and all CI checks, with 0 required approvals** (raise to 1 when a second maintainer joins) | One maintainer, and GitHub doesn't allow approving your own PR | 1 required review with an admin bypass |
| D24 | **P07: abuse controls are a device cap per account (`MAX_DEVICES_PER_USER`, default 5) plus rate limits; no captcha** | No third party in the sign-up path; a captcha can be added if abuse appears | Turnstile on the email link (web widget plus webviews in native apps); email/domain allowlist |
| D25 | **One account per email. A new sign-in method joins it only if it proves control of the mailbox now: a magic link, or Google for gmail.com / the account's Workspace domain. Otherwise sign-in fails with `account_exists`** | "Verified" at GitHub (or Google for a foreign domain) can be years old; a reassigned address must not hand over an account. Only ASCII addresses are accepted, to stop look-alikes | Link on any verified email (account takeover); separate accounts per provider; manual linking UI (later) |
| D26 | **Magic links open a confirmation page; the button's POST consumes the token** | Mail scanners follow links and would burn a one-time GET. Cross-site POSTs are refused | Consume on GET |
| D27 | **Account deletion removes all the user's records and leaves an empty user row marked `deleted_at`** | Meets both "delete all data" and "soft-delete users" | Hard delete; keeping the email |
| D28 | **Anisette provisioning is also capped per IP (10/h) and overall (100/h)** | Per-user quotas reset when an account is deleted and recreated; the shared anisette identity is the asset to protect | Per-user quotas only; blocking re-registration (needs keeping data after deletion) |
| D29 | **Dashboard sign-in buttons are text-only; the sign-in page uses no GitHub or Google logo** | The official brand assets aren't in the repo and unofficial copies aren't allowed (MASTER §6). Add the official assets before a public launch | Icons from a third-party set |
| D30 | **A push subscription records the web session that created it and is deleted when that session ends** | A signed-out or revoked browser must not keep receiving an account's device alerts. Stores one more id per subscription | Client-side unsubscribe only (misses remote revocation) |
| D31 | **P02: LocalDevVPN is driven by `localdevvpn://enable?scheme=orbit` (and `disable`), or its Shortcuts action** (`ControlLocalDevVPNIntent`, runs without opening the app) | Read from its source on 2026-11-27; `scheme` makes it switch back to Orbit. Needs a device check | Asking the user to toggle it by hand |
| D32 | **iOS and desktop sign-ins end on a "Continue as …" page before any code is issued** | Another app can start its own flow and claim `orbit://`; this makes a silent sign-in impossible. A custom scheme can't be made exclusive, and Associated Domains needs a paid account (hard rule 4), so a user who presses the button for an app they didn't open is still exposed | Redirecting straight away |
| D33 | **Each device keeps its newest 1000 history events** | Invented timestamps could otherwise fill the disk; real use stays far below this in 90 days | A per-day cap |
| D34 | **Releases publish only from a tag on a commit in `main`, through the `release` environment; running the workflow by hand is a dry run** | A tag alone shouldn't publish unreviewed code | Publishing on any tag |

## Pending decisions (to be resolved during the named phase)

| ID | Question | Phase |
|---|---|---|
| P01 | **Research done (below); the dependency choice needs the owner's approval** | 3 |
| P03 | winget IDs for Apple Devices / AMDS; fallback when winget is missing (older Windows 10) | 4 |
| P04 | Flatpak USB strategy (host usbmuxd via portal vs `--device=all` + bundled daemon) | 4 |
| P05 | LiveContainer license and integration (built-in store mode) | 5 |
| P06 | Code signing: Apple notarization / Windows Authenticode certificates (cost vs warnings) | 6 |
| P08 | Account pre-hijacking: when a magic link first proves an address, remove the older sign-in methods and sessions (and their devices and sources?), or ask the user to confirm them? Needs a data model change and supersedes part of D25 | 6, before real users |
| P09 | Accept web push endpoints only from known push services (Apple, Google, Mozilla, Microsoft)? Recommended; an unusual browser would lose alerts | 6 |
| P10 | Protecting the shared email and provisioning caps: captcha or proof of work when they run low (D24 ruled a captcha out), or reserve a share for established accounts | 6 |
| P11 | Pairing file protection class, given refresh must run with the screen locked | 3 |
| P12 | Keep secret query tokens in saved source URLs? Encrypt backups? | 6 |

## P01 research (2026-11-27, SideStore `develop` at 4f3d18d)

- SideStore uses **minimuxer** (AGPL-3.0), which wraps both an `idevice` and a `libimobiledevice` gateway. It is not a choice between the two.
- **em_proxy is still used**: SideStore starts it before minimuxer, and minimuxer ships it as a prebuilt binary (checksum-pinned).
- **AltSign was replaced by SideSign** (GPL-3.0 per its README; no `LICENSE` file). It does Apple sign-in, the developer portal and app signing. GPL-3.0 and AGPL-3.0 may be combined.
- SideSign depends on `CodeSignKit`, `GSACryptoKit`, `AnisetteKit` (all AGPL-3.0), and minimuxer on `RemotePairingKit`. **All four are in one person's repositories, created in August 2026, with 0–7 stars, and tracked by branch rather than version.** `GSACryptoKit` handles the Apple ID password.
- SideSign's remote anisette provider can't send our bearer token, but it implements a public protocol, so Orbit can supply its own.
- Pairing files SideStore looks for: `ALTPairingFile.mobiledevicepairing`, `PairingFile_Lockdown.plist`, `PairingFile_RemoteRP.plist`.

**Proposed:** depend on SideSign and minimuxer, pinned to exact commits with a committed `Package.resolved` so no branch can move silently, and review `GSACryptoKit` before it sees a real Apple ID.
**Alternatives:** fork those repositories into this account and pin the forks; or use the older AltSign (no license file, which rules it out unless clarified).
