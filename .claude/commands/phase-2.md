---
description: Phase 2: SvelteKit dashboard PWA with web push
---
Run Phase 2 from docs/ROADMAP.md.

Read first: docs/WEB_DASHBOARD.md, design-system/orbit/MASTER.md, design-system/orbit/pages/dashboard.md, docs/UX_CHECKLIST.md, docs/API.md.
If the UI-UX-Pro-Max skill is available, also run `--stack svelte` and `--domain ux` queries for forms, navigation and push-permission UX. MASTER.md wins on conflicts.

Steps:
1. Build the layout shell (left rail ≥1024px, bottom tabs on mobile, safe-area insets), theme toggle (system/light/dark) and tokens.css.
2. Pages: /login, /, /devices/[id], /activity, /sources, /settings, each with loading skeleton, empty and error states.
3. PWA manifest + service worker (push only), enable/disable/test push, iOS "Add to Home Screen" guidance.
4. Embed the build into the Go server (embed.FS) with SPA fallback.
5. Tests: Vitest, Playwright (mocked API), axe checks in light and dark.
6. Run the ui-reviewer subagent on the diff and fix its findings.
7. Attach screenshots (375/768/1440 × light/dark) to the PR.
