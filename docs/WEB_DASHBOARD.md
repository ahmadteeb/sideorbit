# Web dashboard spec

UI spec: `design-system/orbit/pages/dashboard.md`.

- **Stack:** SvelteKit 2, Svelte 5 runes, TypeScript strict, `adapter-static` (SPA fallback `index.html`), Lucide icons, `tokens.css`.
- **Served by:** orbit-server via `embed.FS` at `/`, with `/api/*` and `/anisette/*` routed to Go handlers first.
- **Auth:** cookie session for the web (HttpOnly, Secure, SameSite=Lax) + CSRF header on mutations. The access JWT is never kept in localStorage.
- **Data:** a thin typed client generated from `docs/API.md` types (hand-written `api.ts` is fine for v1). Server data goes through a small query cache; no global store library.
- **PWA:** `manifest.webmanifest` (name "Orbit", `display: standalone`, theme color = `bg` token), maskable icons, a service worker for push only (no offline caching of API data).
- **Web push:** subscribe with the VAPID public key. The service worker `push` handler shows title/body/url; `notificationclick` focuses or opens the URL.
- **CSP:** emitted by SvelteKit as a `<meta>` tag (script hashes); the server adds `frame-ancestors 'none'`. Fonts are never inlined as `data:` URIs.
- **Embedding:** `server/Dockerfile` builds the dashboard and copies it into `server/internal/web/dist` before compiling.
- **Tests:** Vitest for utils; Playwright E2E against a mocked API; `@axe-core/playwright` on every page in both themes.
