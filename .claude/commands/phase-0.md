---
description: Phase 0: repo foundations, license, design system, CI skeletons
---
Run Phase 0 from docs/ROADMAP.md.

1. Read AGENTS.md, docs/ROADMAP.md (Phase 0), docs/LICENSING.md, design-system/orbit/MASTER.md.
2. Create a task list from the Phase 0 checklist.
3. Create a branch `phase-0-foundations`.
4. Add the AGPL-3.0 `LICENSE` (full official text) and a `NOTICE` crediting SideStore, iloader, anisette-v3-server, LiveContainer and UI-UX-Pro-Max.
5. Create empty-but-buildable skeletons:
   - `server/` (go mod init github.com/<owner>/orbit/server, main.go with /healthz, one test)
   - `web/` (SvelteKit + TS + adapter-static, copy design-system/orbit/tokens.css into src/lib/styles/)
   - `ios/` (XcodeGen project.yml, an Orbit app target with one SwiftUI view using OrbitTheme.swift)
   - `desktop/` (README placeholder; the iloader fork happens in Phase 4)
6. Add workflows: server.yml, web.yml, ios.yml (macos-latest, `brew install xcodegen`, unsigned build, upload Orbit.ipa), desktop.yml (placeholder that passes). Pin actions by SHA.
7. Push, open a PR, `gh run watch`, and fix failures until green.
8. Update the ROADMAP checkboxes. Summarize what the human should verify (branch protection settings).
