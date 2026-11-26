# Web dashboard (SvelteKit PWA): page overrides

Inherits `../MASTER.md`. Overrides: **density = dense-ish for tables** (8–32px spacing inside data tables only), Lucide icons, and IBM Plex Sans + JetBrains Mono loaded with `font-display: swap`.

## Layout

- Mobile-first. Breakpoints are 375 / 768 / 1024 / 1440. No horizontal page scroll; wide tables become stacked cards below 768px.
- Navigation is a left rail at 1024px and wider, and a bottom tab bar (4 items) on mobile, so the installed PWA feels like an app.
- Respect `env(safe-area-inset-*)` in standalone PWA mode.

## Pages

| Route | Content |
|---|---|
| `/` | Status overview: one card per device (name, iOS, VPN/pairing health from the last report, apps with expiry countdowns) |
| `/devices/:id` | Device detail: apps, refresh history timeline, "last seen", remove device (with a confirm that names the device) |
| `/activity` | All refresh events, filterable by device / result. A failed event shows its cause plus a fix link |
| `/sources` | Saved app sources; add by URL with validation and a preview before saving |
| `/settings` | Account, linked logins, **Enable web push** (with iOS instructions: "Add to Home Screen first"), sessions, delete account |
| `/login` | GitHub, Google, email link. Explain what Orbit stores and what it doesn't |

## Charts

The only chart in v1 is the refresh success rate (last 30 days), drawn as a bar chart with a text summary and an accessible table alternative. Don't use color alone to show success or failure; add a pattern or label.

## Web push UX

- Never prompt on page load. Ask only after the user taps "Enable alerts".
- On iOS Safari outside a home-screen app, show the "Add to Home Screen" steps instead of the permission button.
- Show a "Send test alert" button after alerts are enabled.
