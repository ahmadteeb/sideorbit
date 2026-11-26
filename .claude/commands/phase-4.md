---
description: Phase 4: Orbit Setup desktop app (iloader fork) for macOS, Windows, Linux
---
Run Phase 4 from docs/ROADMAP.md.

Read first: docs/DESKTOP_SETUP_APP.md, design-system/orbit/pages/desktop.md, docs/LICENSING.md.

1. Add iloader as a git subtree in desktop/ (`git remote add iloader https://github.com/nab138/iloader.git`) and keep its LICENSE. Build it unmodified in CI first.
2. Resolve P03 (winget IDs and fallback) and P04 (Flatpak USB strategy). Present the options to the human with trade-offs, then record them in DECISIONS.md.
3. Implement the Orbit wizard, login (loopback redirect http://127.0.0.1:<random>/callback with PKCE), anisette gateway + bearer token, driver bootstrap (Windows winget, Linux pkexec helper + udev + bundled usbmuxd), Re-pair only mode, and diagnostics.
4. Packaging in desktop.yml: dmg universal, msi + nsis, AppImage, deb, rpm, Flatpak manifest, AUR PKGBUILD in desktop/packaging/aur.
5. Keep Orbit-specific code in clearly marked modules so upstream rebases stay easy.
6. The PR includes a manual test matrix (Win 11, Win 10, Ubuntu 24.04, Fedora, Arch, macOS 14+) for the human.
