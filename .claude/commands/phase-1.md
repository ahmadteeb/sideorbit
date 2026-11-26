---
description: Phase 1: Go backend, auth, anisette gateway, web push, deploy
---
Run Phase 1 from docs/ROADMAP.md.

Read first: AGENTS.md, docs/ARCHITECTURE.md, docs/API.md, docs/DATA_MODEL.md, docs/SECURITY.md, docs/SELF_HOSTING.md, deploy/*.

Plan the work as sub-PRs and present the plan before coding:
  1a. config + DB + migrations + health
  1b. auth (GitHub, Google, email magic link, token exchange for ios/desktop, refresh rotation)
  1c. devices/apps/events/sources + report endpoint (DisallowUnknownFields)
  1d. anisette gateway (HTTP + WebSocket reverse proxy, bearer auth, rate limits, metadata-only logs)
  1e. web push + scheduler + alerts_sent dedupe
  1f. deploy: Dockerfile (distroless, non-root), compose, Caddyfile, backup.sh

Rules:
- Every handler gets table-driven tests. Test the auth flows against a fake OAuth provider (httptest). The gateway gets a WebSocket proxy test using a fake upstream.
- Add the CI "no-credentials" guard test described in docs/SECURITY.md.
- Resolve pending decision P07 with the human (ask with options), then record it in docs/DECISIONS.md.
- Run `go test ./... && go vet ./... && staticcheck ./...` before each push.
- Run the security-reviewer subagent on 1b and 1d before opening those PRs.

Done when the Phase 1 acceptance criteria in ROADMAP.md pass. Write the exact curl commands the human can use to verify on their VPS.
