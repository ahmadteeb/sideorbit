---
description: Phase 6: security audit, observability, docs, release pipeline, v1.0.0
---
Run Phase 6 from docs/ROADMAP.md.

1. Run the security-reviewer subagent across the whole repo (fresh context, no prior work shown). Triage the findings with the human and fix them.
2. Load-test the anisette gateway with k6 (script in deploy/loadtest/). Tune the rate limits and document them in docs/API.md.
3. Add /metrics, structured log fields and an uptime check example.
4. Finish docs/USER_GUIDE.md (add screenshot placeholders for the human to capture), SELF_HOSTING.md and troubleshooting.
5. Add release.yml: on tag v*, build server image (ghcr, multi-arch), Orbit.ipa (+ LiveContainer variant), all desktop packages, checksums, then a GitHub Release with notes.
6. Resolve P06 (code signing) with the human.
7. Tag v1.0.0 only after the human approves.
