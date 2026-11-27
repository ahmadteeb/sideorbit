# Orbit Setup: desktop app spec

UI spec: `design-system/orbit/pages/desktop.md`.

## Base
- **Fork of `nab138/iloader`** (MIT). It's a Tauri app with Rust (device + signing via the `idevice` ecosystem) and a TypeScript frontend.
- Vendor it as a **git subtree** in `desktop/` with an `upstream` remote so fixes can be pulled. Keep `LICENSE` (MIT) and add an Orbit `NOTICE`.
- Orbit changes: branding, the wizard flow, Orbit login, the anisette gateway URL (+ bearer token), driver bootstrap, "Re-pair only" mode, and LiveContainer+Orbit selection.

## What exists today
Upstream iloader, rebranded, plus: Orbit sign-in (browser or email link, loopback redirect with PKCE), the refresh token in the system keyring, and Apple sign-in using only your server's anisette gateway through a local proxy (D35). The user picks an IPA to install. The wizard, driver bootstrap, Orbit pairing placement and diagnostics below are not built.

### Test matrix (for a human; nothing has been run on real hardware)

| Check | Win 11 | Win 10 | Ubuntu 24.04 | Fedora | Arch | macOS 14+ |
|---|---|---|---|---|---|---|
| Installs and starts | | | | | | |
| Sign in with GitHub/Google, and by email link | | | | | | |
| Sign-in survives a restart; sign-out forgets it | | | | | | |
| iPhone detected | | | | | | |
| Apple ID sign-in with 2FA through your server | | | | | | |
| Install `Orbit.ipa` | | | | | | |

## Driver bootstrap ("no separate driver install")

### macOS
No drivers needed (usbmuxd is built in). Skip the step.

### Windows
1. Detect: the `Apple Mobile Device Service` exists and is running, **or** the Apple Devices app is installed (`Get-AppxPackage *AppleDevices*`).
2. If missing: run `winget install` for **Apple Devices** (Microsoft Store source) silently, with `--accept-package-agreements --accept-source-agreements`. **Verify the exact package ID in Phase 4 (P03).**
3. Fallback if winget is unavailable or fails: offer the "Install iTunes (Apple)" path via its official installer, with clear progress. Never bundle Apple binaries.
4. Re-detect, then continue. Show download size and progress, and allow Cancel.

### Linux
1. Detect: is a usbmuxd socket available at `/var/run/usbmuxd`?
2. If missing: start the **bundled usbmuxd** binary (shipped in the package; GPL, so ship the source offer and notice) via `pkexec` with a fixed helper script that:
   - installs a udev rule granting the `plugdev`/`uaccess` group access to Apple USB devices (vendor `05ac`),
   - installs and starts a systemd unit for the bundled usbmuxd (or runs it once if the user declines the persistent install).
3. One password prompt, a fixed action, and the helper's source shown in the docs.
4. **Flatpak caveat (P04):** a sandbox can't install udev rules or system daemons. Options: require host usbmuxd (most distros ship it) with in-app guidance, or ship a small host helper. Decide in Phase 4.

## Packaging matrix (`desktop.yml` via `tauri-action`)

| OS | Formats | Notes |
|---|---|---|
| macOS | `.dmg` universal (arm64 + x86_64) | Unsigned → first launch: right-click → Open, or System Settings → Privacy & Security → Open Anyway. Notarization is optional (P06) |
| Windows | `.msi` + NSIS `.exe` | Unsigned → SmartScreen "More info → Run anyway". Authenticode is optional (P06) |
| Linux | AppImage, `.deb`, `.rpm`, Flatpak (`flatpak/dev.orbitstore.Setup.yml`) | AppImage is the universal fallback |
| Arch | AUR `orbit-setup-bin` | `PKGBUILD` repackages the `.deb` or AppImage from the GitHub Release. Keep it in `desktop/packaging/aur/`; publish manually or via CI with an AUR SSH key secret |

## Flows
- **Full setup:** login → device → drivers → Apple ID → choose variant (Orbit / LiveContainer+Orbit) → install → pairing → phone checklist.
- **Re-pair only:** device → drivers → generate pairing → place into Orbit (no Apple ID needed).
- **Diagnostics:** "Copy diagnostics" collects OS, app version, device detection state and the last 200 log lines (with tokens and emails masked).
