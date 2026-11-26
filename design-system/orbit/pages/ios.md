# iOS app: page overrides and screen specs

Inherits everything from `../MASTER.md`. Overrides: **system fonts only** (SF Pro / SF Mono) and **SF Symbols only**.

## Navigation: 4 bottom tabs (the data allows 3–5; we use 4)

| Tab | SF Symbol (outline) | Purpose |
|---|---|---|
| **Home** | `house` | Status hero + my apps + next refresh |
| **Browse** | `square.grid.2x2` | App sources (AltStore-format) |
| **Activity** | `clock.arrow.circlepath` | Refresh history and failures |
| **Settings** | `gearshape` | Account, Apple ID, device, VPN, pairing, notifications, Shortcuts |

Use `NavigationStack` per tab, keep each tab's state when switching, and support deep links: `orbit://app/<bundleId>`, `orbit://source?url=`, `orbit://repair`, `orbit://auth`.

## Home

1. **Status hero card:** one sentence plus one icon, in one of these states:
   - "All apps are fresh. Next refresh: tonight 03:00" (primary)
   - "2 apps expire in 31h. Refresh now" (warning, with a CTA)
   - "Turn on LocalDevVPN to refresh" (warning, with a CTA that opens LocalDevVPN, i.e. `localdevvpn://`; verify the URL scheme in Phase 3)
   - "Pairing file needs replacing" (danger, with a CTA that starts the re-pair flow)
2. **Connection strip:** shows VPN ● Connected / ○ Off and Pairing ✓ / ✗ as icon + text chips. Tapping one opens the fix-it sheet.
3. **App slots meter:** "2 of 3 app slots used" (free account). It's a segmented bar with a text label, and it explains LiveContainer if the user is full.
4. **My apps list:** icon, name, version (mono), a freshness chip, and a trailing "Refresh" button (44pt). Swipe actions mirror the items in the context menu.
5. Pull-to-refresh fetches status **only**. It never starts a re-sign by itself.

## Install / refresh progress

- Shows a bottom sheet with named steps: Preparing → Signing → Uploading → Installing → Done.
- Each step shows a check when done. Errors show inline with a cause, a fix and a "Details" disclosure.
- The phone gives a haptic `.success` or `.error` at the end.
- Keep the screen awake during installs (`isIdleTimerDisabled`) and restore the setting afterwards.

## Onboarding (≤ 6 screens, skippable where safe, progress dots + "Step n of 6")

1. Welcome: what Orbit does, in one line.
2. Sign in: GitHub / Google / email link, using official brand buttons. Explain that this is the Orbit account, not the Apple ID.
3. Apple ID: an explainer ("Stays on this iPhone in the Keychain. Never sent to Orbit servers.") and a field that supports password managers and paste. The 2FA code field uses `.oneTimeCode`.
4. Pairing file: detect it if the Orbit Setup desktop app already placed one; otherwise show an "Import pairing file" document picker.
5. LocalDevVPN: an App Store link, a "Check connection" button and a live status.
6. Notifications + Shortcuts: ask for notification permission only here, with the reason ("We'll warn you before apps expire"). Offer "Add refresh automation".

## Settings sections

Account · Apple ID (masked email, team ID in mono, "Sign out of Apple ID") · This device (name, iOS version, app slot usage) · Connection (VPN, pairing, **Re-pair**) · Refresh (schedule, Wi-Fi only, low-power behavior) · Notifications · Shortcuts · Sources · Advanced (anisette server URL, logs export) · About (licenses: AGPL notice, credits for SideStore/iloader/anisette-v3-server).

## Empty / error states

Every list has an empty state with one SF Symbol, one sentence and one CTA. There are no blank screens.
