# Licensing notes (not legal advice)

| Component | License | Implication for Orbit |
|---|---|---|
| **Orbit (this repo)** | **AGPL-3.0** | Required because we link SideStore code. The source must stay public, including for network users of modified versions |
| SideStore and its libraries (AltSign, minimuxer, em_proxy…) | AGPL-3.0 (verify each in Phase 3) | Keep notices and credit them in About |
| Go modules: chi, pgx, goose, golang-jwt, webpush-go | MIT | Compatible with AGPL-3.0; license texts ship with the modules |
| Go modules: golang.org/x/oauth2, golang.org/x/time, google/uuid | BSD-3-Clause | Compatible with AGPL-3.0 |
| `coder/websocket` (tests only) | ISC | Not part of the server binary |
| `prometheus/client_golang` and its libraries | Apache-2.0 (protobuf BSD-3-Clause) | Compatible with AGPL-3.0 |
| `grafana/k6` | AGPL-3.0 | A tool run by operators; not part of Orbit |
| `nab138/iloader` | MIT | Can be forked into `desktop/`; keep its copyright notice |
| `Dadoum/anisette-v3-server` | **No license file found in the repo** (checked Nov 2026) | We **run the unmodified upstream Docker image** and don't copy or modify its code. Don't vendor or fork it without permission from the author |
| LiveContainer | Verify in Phase 5 (P05) | Decide the integration approach after checking |
| usbmuxd (bundled on Linux) | GPL (verify version) | Ship it as a separate binary with notice + source offer |
| UI-UX-Pro-Max skill data | MIT | Design decisions are derived from it; credit it in `design-system/orbit/MASTER.md` |
| IBM Plex Sans, JetBrains Mono (via @fontsource) | SIL OFL 1.1 | Bundled in the dashboard with their license files (`web/static/licenses/`) |
| Lucide (`@lucide/svelte`) | ISC | Dashboard icons |
| Playwright (Apache-2.0), `@axe-core/playwright` (MPL-2.0) | tests only | Not shipped |
| Apple drivers (Windows) | Apple proprietary | **Never bundle.** Installed from the official source on the user's machine (D15) |
| GitHub / Google logos | Brand guidelines | Use the official sign-in button assets unmodified |

Add the AGPL-3.0 full text as `LICENSE` in Phase 0 (GitHub → Add file → "Choose a license template" → GNU AGPLv3, or `curl https://www.gnu.org/licenses/agpl-3.0.txt -o LICENSE`).
