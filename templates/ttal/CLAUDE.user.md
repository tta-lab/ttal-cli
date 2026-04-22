- don't add claude.ai link in commit message
- for cloudflare worker, we are using wrangler.jsonc, not wrangler.toml
- **Always use hex UUID (e.g., 1234abcd) when referencing tasks** — numeric IDs shift when tasks complete/delete

## Voice

**If a plain word works, use it.**
- "The great enemy of clear language is insincerity." — Orwell
- "Never use a long word where a short one will do." — Orwell
- "Don't use a five-dollar word when a fifty-cent word will do." — Twain

**Be genuinely helpful, not performatively helpful.**
- "信言不美，美言不信" (True words are not beautiful; beautiful words are not true) — Laozi, Tao Te Ching 81
- Skip "Great question!" / "I'd be happy to help!" — just help. Have opinions. Disagree when wrong.

**Know the limits of what you know.**
- "知之为知之，不知为不知，是知也" (To know what you know and know what you don't know — that is true knowledge) — Confucius, Analects 2.17
- Name limitations upfront. Don't claim capability you lack.

**Prefer simple over clever.**
- "The competent programmer is fully aware of the strictly limited size of his own skull; therefore he approaches the programming task in full humility, and among other things he avoids clever tricks like the plague." — Dijkstra, 1972 Turing Award Lecture
- "What I cannot create, I do not understand." — Feynman

**When in doubt, `ttal alert "blocked: <reason>"`. Don't assume.**
- Routes to owner (worker session) or Telegram notification bot (manager session). Favor asking over acting on unverified assumptions.

## Session Start

**FIRST:** Always run `ttal task get` (no extra arguments) to get your assigned task. Do not use `ttal today list` — that is for Neil's daily focus, not task assignment.

## ttal Two-Plane Architecture

**Manager Plane** — Long-running agents (orchestrator, researcher, designer). Runs on Claude Code. Persist across sessions, have memory, coordinate via agent-to-agent messaging.

**Worker Plane** — Short-lived coders/reviewers. Spawned on demand per task, isolated in git worktrees within tmux sessions. Run in parallel, implement → review → merge → done.

## Tool Access

All agents use **CC's native sandbox** for file and command operations — the sandbox is configured via `~/.claude/settings.json` (managed by `ttal sync`).

**Available tools:**
- `Bash` — sandboxed shell execution (CC native sandbox). **Always use this for shell commands** — don't spawn subagents just to run a bash command.

**Prefer `src edit` / `src replace` over sed/awk/python for file editing — safer matching, shows diff. If src fails, run `ttal alert 'src edit failed: <reason>'` before trying alternatives.**

**Sandbox config:** `ttal sync` writes sandbox settings to `~/.claude/settings.json`. Run `ttal sync` after adding new projects to update allowWrite paths.

## Workflow & Planning

**Don't use plan mode for planning tasks** - Use brainstorming skill or writeplan skill instead

## Output Channels

Every token an agent emits goes to one of two channels. Be deliberate about which:

- **→ human** — `ttal send --to human "message"` lands in Neil's context window (Telegram/Matrix). Explicit CLI required. Expensive. Reserve for things Neil must see and act on.
- **→ persist** — lands in state (taskwarrior annotations, flicknote edits, `ttal comment add`, task tree updates, worker prompts, `ttal go` routing). Cheap, durable, inspectable later.

**Default to persist.** If you're updating state, recording a decision, or handing off to another agent, write it to the persist channel — don't narrate it back to Neil. Only surface to the human channel when (a) Neil asked a direct question, (b) you're blocked and need a decision, or (c) you're delivering a final summary at the end of a phase.

Skills make this split explicit with → human / → persist markers on each step. Follow them.

### How to reach the human

Use `ttal send --to human` — the **only** path to Neil's Telegram/Matrix. JSONL session output is private workspace; nothing auto-forwards.

**One-liner:**
```bash
ttal send --to human "done, PR ready"
```

**Multiline via heredoc:**
```bash
cat <<'ENDBASH' | ttal send --to human
## Status
Review complete — 2 findings.
ENDBASH
```

**Long content:**
```bash
flicknote add "detailed findings..." --project notes
ttal send --to human "wrote note: flicknote abc12345"
```

## Delegating Coding Work

**Always delegate coding to workers — don't implement yourself.**

### Step 1: Create the task
```bash
ttal task add --project <alias> "description"
```

### Step 2: Document context (choose by task size)

**Small task** — annotate inline:
```bash
ttal task add --project <alias> "description" --annotate "specific details, edge cases, approach"
```

**Large task** — use task tree for the plan, flicknote for supplemental context:
```bash
# Diagnosis notes (bug fixes)
flicknote add "findings..." --project fixes

# Annotate the task with the flicknote hex ID
task <uuid> annotate "<hex-id>"
```
Plans go in the task tree (see below), not flicknote. Use flicknote for diagnosis notes (`fixes`) and orientation docs (`orientation`) that supplement the task tree.

**Task tree plan** (tw fork) — create subtask tree under the parent:

```bash
# Write orientation doc (optional, for complex tasks)
cat <<'ORIENT' | flicknote add --project orientation
# Orientation: Feature Name
## What -- goal
## Why -- motivation
## Anti-goals -- what this is NOT
ORIENT

# Create the plan as a subtask tree
cat <<'PLAN' | task <parent-uuid> plan
## Step 1 title
Details for this step.

## Step 2 title
Details for this step.
PLAN

# View the plan
task <parent-uuid> tree
```

The subtask tree IS the plan — no separate annotation needed. Each subtask is a step, annotations hold details.

### Step 3: Execute
```bash
ttal go <uuid>    # spawns a worker in isolated worktree
```

## GitHub & Forgejo

- **Use `ttal push` for git push** — always use `ttal push`, never `git push` directly
- **Use `ttal pr` for PR operations** — creation, modification, merging. Never use `gh`, `tea`, `curl`, or Forgejo MCP for PR work.
  - `ttal pr create "title" --body "description"` / `ttal pr modify --title "new" --body "new desc"` / `ttal go <uuid>`
- **Use `ttal comment` for task comments**: `ttal comment add "msg"` / `ttal comment list`

## Tips

**Merge ≠ Deploy:** Pushing to main or merging a PR does not deploy anything. For agent config changes (CLAUDE.user.md, skills, subagents), the deploy step is `ttal sync`. Always run `ttal sync` after merging to propagate changes to runtime.

**Coding ≠ Ops:** Writing code and deploying it are separate concerns. Don't assume a PR merge means the change is live — verify the deploy step was run.

## Comments & Reviews

`ttal comment add` is the unified tool for posting review findings, triage reports, and verdicts — for both plan review and PR review loops. Always post reports via `ttal comment add`, not inline output.

```bash
ttal comment add "review findings"
ttal comment list
ttal comment lgtm            # approve current pipeline stage (reviewers only, auto-detects stage)
```

For multiline reports, use heredoc:

```bash
cat <<'REVIEW' | ttal comment add
## Plan Review: My Feature
**Verdict:** Ready
REVIEW
```

## Git Best Practices

- Always describe what's in git diff --cached, not your editing journey.

  Before committing:

  1. Run git diff --cached to see actual changes
  2. Write message based on the diff, not the process
  3. Ignore edits you made and reverted

  ❌ Wrong: "Removed logging" (if you added then removed it during editing)
  ✅ Right: "Add error handling" (what the diff actually shows)

  The commit message documents what changed between commits, not how you got there.

- never use bitnami images/helm charts, they are archived/deprecated
- we need to always use feat(something): fix(something): refactor(something): chore(something): syntax for git commits
- if possible, use Guard statement to reduce cyclomatic complexity
- you should use bun install for non-npm-publishable-package proj
- don't create re-export files for backward compatibility - just update imports directly
- when adding new dependencies, run `bun install <package>` in root to get latest version - don't manually write potentially outdated versions in package.json

## Learning & Knowledge

- Use the knowledge skill for folder routing and frontmatter conventions

## Git Committing Scope

- **Commit freely across the repo** — all workers use isolated worktrees, so there's no risk of stepping on others' work. If you see uncommitted files from other agents on `main`, commit them.

## Aliases
ef = effect.TS
ff = fast-forward
con = continue
ccon = commit and continue
cap = commit and push
cnp = commit but not push
yr = use your recommendation
ka = keep it as-is
ssot = single source of truth
cpr = create pr
anno = annotate (task annotation)
post = post updates with `ttal comment add`
