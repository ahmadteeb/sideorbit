# CLAUDE.md

@AGENTS.md

## Claude Code workflow for this repo

- **Start every session** by reading `docs/ROADMAP.md` to find the current phase, then the specs that phase links to. Use the phase slash commands in `.claude/commands/` (`/phase-0` … `/phase-6`).
- **Plan before code** for anything spanning more than one package: write the plan as a checklist in the PR description (or a task list), then implement.
- **Use a task list** for each phase, and tick items as they finish so a returning human sees progress.
- **Small, verified steps:** after each meaningful change, run the relevant tests from AGENTS.md §4. Don't stack unverified changes.
- **Hand iOS and desktop builds to CI:** push the branch, watch `gh run watch`, read failing logs with `gh run view --log-failed`, fix, repeat.
- **UI work:** read `design-system/orbit/MASTER.md` + `pages/<surface>.md` first. When done, run the `ui-reviewer` subagent (`.claude/agents/ui-reviewer.md`) on the diff and fix what it finds.
- **Security-sensitive work** (auth, tokens, anisette gateway, anything touching credentials or pairing files): run the `security-reviewer` subagent before opening the PR.
- **UI-UX-Pro-Max skill:** if it's installed in this environment, use it (`--design-system`, `--domain ux`, `--stack swiftui|svelte`). If its output conflicts with `MASTER.md`, MASTER wins unless the human approves a change. Record approved changes in `docs/DECISIONS.md`.
- **Never** run `git push --force` to `main`, delete branches you didn't create, or change files outside the repo.
- **Ask the human** when a decision is hard to reverse or ambiguous: data model changes after Phase 1 ships, license-affecting dependencies, anything that changes what data the server stores.

## Definition of done (every PR)

- [ ] Tests added and passing locally (server/web) and in CI (all workflows green)
- [ ] `docs/` updated if behavior, API or decisions changed
- [ ] UI: pre-delivery checklist passed (`docs/UX_CHECKLIST.md`), with light + dark screenshots in the PR
- [ ] No secrets, no credentials logged, and no new data stored server-side without a DECISIONS.md entry
- [ ] "What to test on a device" section in the PR description (for iOS/desktop)
