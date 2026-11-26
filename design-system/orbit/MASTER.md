# Orbit Design System: MASTER (single source of truth)

> Built from the **UI-UX-Pro-Max** skill data (MIT, nextlevelbuilder/ui-ux-pro-max-skill).
> These picks were made by hand from the data files: `products.csv` row 81 "Developer Tool / IDE", `colors.csv` row 81, `ui-reasoning.csv` row 81, `typography.csv` row 9 "Developer Mono", and `styles.csv` "Dark Mode (OLED)" + "Minimalism & Swiss Style". The skill's generator script was **not** run.
> If you have the skill installed locally, you can regenerate the system and compare:
> `python <skill>/scripts/search.py "iOS developer utility app store dark modern" --design-system -p "Orbit"`
>
> **Rule:** page files in `pages/` override this file only for the things they explicitly list. Do not put raw hex values in components. Use the tokens in `tokens.json` / `tokens.css` and the Swift `OrbitTheme`.

## 1. Product read

| Field | Value |
|---|---|
| Product type | Developer utility / personal app store (sideloading + auto-refresh) |
| Audience | Tech-curious iPhone users who aren't always developers. Users are often nervous about "will my apps expire?" |
| Core emotion to design for | **Calm confidence.** The user should always see at a glance that everything is fresh and nothing is required of them right now. |
| Platforms | iOS (SwiftUI), web dashboard (SvelteKit PWA), desktop setup app (Tauri, Mac/Win/Linux) |

## 2. Style

- **Primary style:** Dark Mode (OLED) + Minimalism & Swiss Style.
- **Secondary:** Flat Design + Micro-interactions (status changes, refresh progress).
- **Default theme:** follow the system setting. Dark is the "hero" theme in marketing and screenshots. **Light mode must still be fully supported and tested separately.** The reasoning data calls light-only-by-default an anti-pattern for dev tools, and the pro rules forbid testing only one theme.
- **Anti-patterns (from the reasoning data and pro rules):** slow performance, complex onboarding, emoji used as icons, mixing filled and outline icons at the same level, gray-on-gray text, a single animation duration for everything.

## 3. Color: semantic tokens

The palette is based on `colors.csv` row 81 ("Code dark + run green"), adjusted for contrast. **Green means fresh/signed**, the product's core status. Every pair below was checked (WCAG ratio in brackets).

| Token | Dark | Light | Use |
|---|---|---|---|
| `bg` | `#0B1120` | `#F8FAFC` | App / page background |
| `surface` | `#151C2E` | `#FFFFFF` | Cards, sheets, list rows |
| `surface-2` | `#1E2740` | `#F1F5F9` | Nested surfaces, input fill, skeletons |
| `text` | `#F8FAFC` (18.0 / 16.2) | `#0F172A` (17.1 / 17.9) | Primary text |
| `text-muted` | `#A3AFC2` (8.5 / 7.7) | `#475569` (7.2 / 7.6) | Secondary text, captions |
| `border` | `#2A3550` | `#E2E8F0` | Decorative dividers only (no meaning) |
| `border-strong` | `#64748B` (≥3.5) | `#64748B` (≥4.5) | Input and control outlines (non-text ≥3:1) |
| `primary` | `#22C55E` | `#15803D` | Primary CTA, "Fresh" status, progress |
| `on-primary` | `#0B1120` (8.3) | `#FFFFFF` (5.0) | Text and icons on primary |
| `warning` | `#FBBF24` (10.2) | `#B45309` (5.0) | "Expiring soon" (< 48h), VPN off |
| `danger` | `#F87171` (6.1) | `#B91C1C` (6.5) | Expired, refresh failed, destructive actions |
| `info` | `#60A5FA` (6.7) | `#1D4ED8` (6.7) | Links, focus ring, neutral info |
| `focus-ring` | `#60A5FA` | `#1D4ED8` | 2px ring, 2px offset; always visible |
| `scrim` | `rgba(2,6,23,0.72)` | `rgba(15,23,42,0.48)` | Modal backdrop. Check it against the real background. |

**Status language (color is never the only signal; always pair it with an icon and a word):**

| State | Color | SF Symbol / Lucide | Label |
|---|---|---|---|
| Fresh (> 48h left) | primary | `checkmark.seal.fill` / `badge-check` | "Fresh · 5d left" |
| Expiring (≤ 48h) | warning | `clock.badge.exclamationmark` / `clock-alert` | "Expires in 31h" |
| Expired | danger | `xmark.seal.fill` / `badge-x` | "Expired" |
| Refreshing | info | `arrow.triangle.2.circlepath` / `refresh-cw` (rotating; static if reduced motion) | "Refreshing…" |
| Needs action | warning | `exclamationmark.triangle.fill` / `triangle-alert` | "Turn on VPN", "Re-pair device" |

## 4. Typography

The pairing is based on "Developer Mono" (`typography.csv` row 9): **IBM Plex Sans** (UI) + **JetBrains Mono** (technical values).

| Platform | UI font | Mono (bundle IDs, versions, UDIDs, logs) |
|---|---|---|
| iOS | **SF Pro (system)**. Required for Dynamic Type; don't bundle Plex on iOS | SF Mono (`.monospaced()`) |
| Web / Desktop | IBM Plex Sans 400/500/600 | JetBrains Mono 400/500 |

Type scale (web px / iOS text style):

| Role | Web | iOS | Weight |
|---|---|---|---|
| Display | 32/40 | `.largeTitle` | 600 |
| Title | 22/28 | `.title2` | 600 |
| Headline | 17/24 | `.headline` | 600 |
| Body | 16/24 (min 16) | `.body` | 400 |
| Callout | 15/22 | `.callout` | 400 |
| Caption | 13/18 (never < 12) | `.caption` | 500 |
| Mono | 14/20 | `.footnote.monospaced()` | 400 |

Line height for body text is about 1.5. Set `font-variant-numeric: tabular-nums` / `.monospacedDigit()` on countdowns so they don't jitter as they tick.

## 5. Spacing, radius, elevation

- **Spacing (4/8 rhythm, standard density):** 4, 8, 12, 16, 24, 32, 48, 64. Screen gutter is 16 on phone, 24 on large phone / landscape, 32 on tablet / desktop.
- **Section rhythm:** 16 within a card, 24 between cards, 32 between sections, 48 between page-level blocks.
- **Radius:** `sm` 8 (chips, inputs) · `md` 12 (buttons) · `lg` 16 (cards) · `xl` 24 (sheets) · app icons use the iOS squircle mask, never a raw radius.
- **Elevation:** in dark mode use surface tone steps, not shadows. In light mode use `0 1px 2px rgba(15,23,42,.06), 0 4px 12px rgba(15,23,42,.06)`.
- **Max text width:** 680px for long text on tablet/desktop.

## 6. Iconography

- **iOS:** SF Symbols only, with hierarchical rendering. **Web / Desktop:** Lucide only, 1.75px stroke.
- Sizes are tokens: `icon-sm` 16, `icon-md` 20 (web) / 24pt (iOS), `icon-lg` 28.
- Use **outline icons in navigation and fill icons only for status badges.** Never mix the two at the same level.
- **No emoji as icons, anywhere.**
- Use brand logos (GitHub, Google) only as official assets, unmodified, on the sign-in buttons.

## 7. Motion

| Token | Duration | Easing | Use |
|---|---|---|---|
| `motion-instant` | 100ms | ease-out | Press feedback (opacity / scale 0.97, no layout shift) |
| `motion-fast` | 150ms | ease-out | Toggles, chips, small state changes |
| `motion-base` | 220ms | ease-out enter / ease-in exit (exit ≈ 70% of enter) | Cards, banners |
| `motion-sheet` | iOS spring (response 0.35, damping 0.85) | — | Sheets, navigation |
| `motion-progress` | continuous | linear | Refresh spinner / progress ring |

- **Reduced motion:** turn off rotation and slide animations, use crossfades, and keep a static icon plus a text label.
- Motion must carry meaning: a status change animates once, and the app never loops animations for decoration.

## 8. Interaction & accessibility (non-negotiable)

- Touch targets are ≥ 44×44pt on iOS (enlarge the hit area if the icon is smaller), with ≥ 8pt between targets.
- Every tappable element gives pressed feedback within 80–150ms, with no layout shift.
- Icon-only buttons get an `accessibilityLabel` / `aria-label`. Decorative icons next to text are hidden from VoiceOver.
- Support Dynamic Type up to AX5 without truncating important info. Status rows wrap; they never clip.
- Forms have visible labels (never placeholder-only labels), errors next to the field, and support for paste and password managers. Validate when a field loses focus and on submit.
- Any operation over 300ms shows a skeleton or progress indicator. Long operations (install, refresh) show named steps ("Signing…", "Installing…").
- Every destructive action (revoke certificate, delete device, remove app) needs a confirmation that names the thing being removed.
- No gesture-only actions: every swipe action also exists as a visible button or menu item.

## 9. Voice & copy

- Plain words, not jargon. Say "Your apps are up to date", not "Provisioning profiles valid".
- Always say **what happens next** and **what the user must do**, if anything. Example: "Orbit will refresh tonight. Nothing to do."
- Explain errors with a cause and a fix, for example: "Couldn't reach your iPhone. Turn on LocalDevVPN, then tap Retry."
- Technical details (bundle ID, team ID, logs) go in a "Details" disclosure in mono text.
