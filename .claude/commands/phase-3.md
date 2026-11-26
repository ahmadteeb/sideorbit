---
description: Phase 3: SwiftUI iOS app (sign-in, signing, install, refresh engine)
---
Run Phase 3 from docs/ROADMAP.md.

Read first: docs/IOS_APP.md, design-system/orbit/pages/ios.md, MASTER.md, docs/UX_CHECKLIST.md, docs/SECURITY.md, AGENTS.md hard rules 1 and 4.

Step 0, research spike (do this first, report back, and wait for approval):
- Inspect SideStore's current default branch: which device library (minimuxer vs idevice), whether em_proxy is still used with LocalDevVPN, how the pairing file is located, and how AltSign is consumed (SPM?). Note licenses.
- Check the LocalDevVPN URL scheme / App Intents.
- Propose the dependency plan and record P01/P02 in docs/DECISIONS.md.

Then build in sub-PRs: 3a project + design system + Orbit login · 3b anisette client + Apple ID + certificates · 3c pairing + VPN probe + device connection · 3d sources + install/remove · 3e refresh engine + App Intent + BGTasks + local notifications · 3f server reporting · 3g onboarding + settings + re-pair flow.

Rules:
- No entitlements beyond defaults. Keychain items per docs/IOS_APP.md. Never log credentials.
- Every view model gets XCTest coverage. Add #Preview for light/dark/AX5 states.
- CI (ios.yml) must be green. You cannot run the app, so each PR lists "What to test on a device" step by step.
- Run the ui-reviewer subagent for each UI PR and the security-reviewer for 3b/3c.
