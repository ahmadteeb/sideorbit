---
description: Review the current UI diff against the Orbit design system and UX checklist
---
Use the ui-reviewer subagent on the current branch's diff against main (`git diff main...HEAD`). Report findings grouped by severity (CRITICAL / HIGH / MEDIUM), each with file:line, the rule it breaks (MASTER.md section or UX_CHECKLIST item) and a concrete fix. Then apply the fixes for CRITICAL and HIGH, and list the MEDIUM ones for the human.
