---
name: sp-executing-plans
description: Use when you have a written implementation plan to execute in a separate session with review checkpoints
---

# Executing Plans

## Overview

Load plan, verify environment, execute all tasks continuously, create PR when done.

**Core principle:** Plans are pre-approved by the architect. Execute them fully without pausing for feedback. Stop only when blocked.

**Announce at start:** "I'm using the executing-plans skill to implement this plan."

## The Process

### Step 0: Verify Environment

Before touching any code, confirm you're in the right place:

1. **Check your working directory:**
   ```bash
   pwd
   ```
   You should be in a git worktree (e.g. `/path/to/project/.worktrees/<task-name>`), NOT the main workspace.

2. **Check you're on the right branch:**
   ```bash
   git branch --show-current
   ```
   Should be `worker/<task-name>`, NOT `main` or `master`.

**If anything is wrong — STOP.** Do not try to fix the environment yourself. Alert Neil:
```bash
ttal alert "wrong workdir: expected worktree but pwd is <path>"
```

**Worktree rules:**
- **STAY in this directory** — do NOT `cd` to the parent/main workspace
- This is an isolated workspace for your task — all work happens here
- When done: commit, push, and create PR with `ttal pr create "title" --body "description"`

### Step 1: Load Context and Review Plan

1. **Get the task context** — the worker environment is pre-configured, so just run:
   ```bash
   ttal task get
   ```
   This returns the full task prompt with inlined docs. Do NOT pass a UUID — the env var handles it.

2. **Read the plan file** (path is in the task annotation or description)

3. **Verify you're in the right project:**
   Does the codebase you're sitting in match what the plan describes? If the plan talks about a different repo or project than what's in `pwd` — you were spawned into the wrong project (likely a bad `project_path` in the task).
   ```bash
   ttal alert "wrong project: plan describes <project> but spawned in <actual path>"
   ```

4. **Review plan critically** — does it make sense?
   - Are the steps clear and actionable?
   - Are there missing dependencies or prerequisites?
   - Does the plan reference files/modules that actually exist?

5. **If the plan is problematic** — alert Neil with specifics:
   ```bash
   ttal alert "plan issue: <what's wrong — missing steps, contradictions, unclear scope>"
   ```

6. If no concerns: Create TodoWrite and proceed

### Step 2: Execute All Tasks

Execute every task in the plan **continuously** — do not pause between tasks or batches.

For each TodoWrite item:
1. Mark as in_progress in TodoWrite
2. Follow each step exactly (plan has bite-sized steps)
3. Run verifications as specified (build, test)
4. Commit as specified in the plan
5. Mark as completed in TodoWrite
6. Move to the next item immediately

### Step 3: Complete Development

After all tasks complete and verified:

1. **Verify all tests pass**

2. **Push:**
   ```bash
   git push
   ```

3. **Create PR:**
   ```bash
   ttal pr create "PR title" --body "description of changes"
   ```

## When to Stop

**STOP executing and alert when:**
- Hit a blocker mid-batch (missing dependency, test fails, instruction unclear)
- Plan has critical gaps preventing starting
- You don't understand an instruction
- Verification fails repeatedly
- Environment is wrong (worktree, branch, project)

Use `ttal alert` to notify Neil, then wait for new messages:

```bash
ttal alert "blocked: test suite fails on auth module — missing test fixtures"
ttal alert "plan issue: step 3 references utils/foo.ts which doesn't exist"
```

Don't guess your way through blockers — alert and wait.

## When to Revisit Earlier Steps

**Return to Review (Step 1) when:**
- Partner updates the plan based on your feedback
- Fundamental approach needs rethinking

**Don't force through blockers** — stop and alert.

## Remember
- Verify environment first — wrong workdir wastes everyone's time
- Always use `ttal task get` (no UUID) to load context
- Review plan critically before executing
- Follow plan steps exactly
- Don't skip verifications
- Reference skills when plan says to
- Execute continuously — do NOT pause between tasks for feedback
- Stop only when blocked — use `ttal alert` to flag issues
- Never start implementation on main/master branch

## Integration

**Required workflow skills:**
- **sp-planning** - Creates the plan this skill executes
- **knowledge** - Vault conventions (frontmatter, folder structure)
