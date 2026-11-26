# UX pre-delivery checklist

Run this before marking any UI task done. It's adapted from the UI-UX-Pro-Max skill's pro rules (MIT) and the Orbit design system. If the skill is installed, also read its `references/pro-rules.md` and `references/quick-reference.md` §1–§3.

## Process
- [ ] Read `design-system/orbit/MASTER.md` + the surface's `pages/*.md` before building
- [ ] Checked on a small phone (375pt / iPhone SE), a large phone, a tablet, and landscape
- [ ] Tested with **Reduce Motion** on and **largest Dynamic Type** (AX5) / browser zoom 200%
- [ ] Tested **light and dark** separately (don't assume one carries over)

## Visual
- [ ] Tokens only, with no raw hex / ad-hoc colors in components
- [ ] One icon family per surface (SF Symbols on iOS; Lucide on web/desktop), consistent stroke and size tokens, no emoji as icons
- [ ] Outline icons for navigation and filled icons for status only; never mixed at the same level
- [ ] Spacing follows the 4/8 rhythm and section tiers 16/24/32/48
- [ ] Status always shows **icon + word + color**

## Interaction
- [ ] Every target is ≥ 44×44pt with ≥ 8pt between targets
- [ ] Press feedback within 80–150ms, without layout shift
- [ ] Operations over 300ms show progress; long operations show named steps
- [ ] Disabled controls look disabled and are not interactive
- [ ] No gesture-only actions (every swipe has a button or menu equivalent)
- [ ] Destructive actions confirm and name the thing being removed

## Contrast
- [ ] Body and secondary text ≥ 4.5:1 in both themes
- [ ] Control borders and meaningful icons ≥ 3:1
- [ ] Modal scrim checked against the real background

## Layout
- [ ] Safe areas are respected (notch, home indicator, PWA standalone)
- [ ] Scroll content is never hidden under fixed bars
- [ ] Long text is ≤ 680px wide on large screens
- [ ] No horizontal page scroll on the web at 375px

## Accessibility
- [ ] Icon-only controls have labels; decorative icons are hidden from VoiceOver / screen readers
- [ ] Focus order = visual order; focus is visible (2px ring) and never hidden behind sticky UI
- [ ] Form fields have visible labels, inline errors and hints; password managers, paste and one-time-code autofill work
- [ ] Dynamic updates (refresh progress, status changes) are announced
- [ ] Color is never the only indicator
