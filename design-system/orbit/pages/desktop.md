# Orbit Setup (desktop, Tauri): page overrides

Inherits `../MASTER.md`. It's a single-purpose wizard: a fixed window of about 720×560 that can be resized, with keyboard navigation everywhere (Tab order = visual order, Enter = primary action, Esc = back/cancel).

## Wizard steps (a progress rail on the left, the step on the right)

1. **Connect your iPhone:** a live device detection card (name, iOS version). If no device is found, show the cause and a fix: "Unlock your iPhone and tap Trust", "Try another cable/port".
2. **Prepare drivers** (automatic and only shown when needed):
   - Windows: "Installing Apple device support (~X MB)…" with progress and a Cancel button.
   - Linux: "Orbit needs permission to talk to USB devices. You'll be asked for your password once."
   - macOS: this step is skipped.
3. **Sign in to Apple ID:** used locally on this computer only to sign the first install. It's never stored after setup unless the user opts in.
4. **Install Orbit:** named steps with a check after each one.
5. **Pairing file:** generated and placed into Orbit automatically, with a fallback "Save pairing file…" button.
6. **Done:** a checklist for the phone: enable Developer Mode, trust the developer profile, install LocalDevVPN, open Orbit. Each item shows the exact Settings path.

## Rules

- A long step can always be cancelled, and an error never leaves the user on a dead end. Every error offers **Retry** plus **Copy diagnostics**.
- Use OS-native window controls on each platform, and match the OS light/dark setting.
- Sign-in fields support password managers and paste.
