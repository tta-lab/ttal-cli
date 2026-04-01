- don't add claude.ai link in commit message
- for cloudflare worker, we are using wrangler.jsonc, not wrangler.toml
- **Always use hex UUID (e.g., 1234abcd) when referencing tasks** — numeric IDs shift when tasks complete/delete

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

**Large task** — write a plan in flicknote, then annotate with the note ID:
```bash
# Project name MUST contain "plan" or "fix" so workers can find it
flicknote add "# Plan: ..." --project myproject-plan
# or
flicknote add "# Fix: ..." --project myproject-fix

# Then annotate the task with the flicknote ID
ttal task add --project <alias> "description" --annotate "plan: flicknote/<id>"
```

Workers automatically look for flicknote notes in projects named `*plan*` or `*fix*`. Using this naming convention ensures workers see your context without you needing to paste it manually.

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

- **Use `ttal pr` for PR operations** — creation, modification, merging. Never use `gh`, `tea`, `curl`, or Forgejo MCP for PR work.
  - `ttal pr create "title" --body "description"` / `ttal pr modify --title "new" --body "new desc"` / `ttal go <uuid>`
- **Use `ttal comment` for task comments**: `ttal comment add "msg"` / `ttal comment list`

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
