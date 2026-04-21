---
title: PR Review Workflow
description: Owner-first PR review with optional specialized reviewer run
---

ttal supports an owner-first PR workflow where the task owner reviews before specialized reviewers engage.

## The pipeline

```
Worker implements task
    ↓
ttal pr create "feat: add auth"
    ↓
Owner notified via ttal send
    ↓
Owner runs skill get sp-review-against-plan
    ↓
Owner verdict:
  LGTM      → ttal go <uuid>
              ↓
              pr-review-lead spawns for specialized review pass
  NEED_WORK → ttal send --to <uuid>:coder (blockers)
              ↓
              Worker fixes, owner re-reviews
```

## Creating PRs

From a worker session:

```bash
# Create PR with title
ttal pr create "feat: add user authentication"

# Create PR with description
ttal pr create "fix: timeout bug" --body "Fixes #42"
```

The PR context is auto-resolved from the worker's environment: `TTAL_JOB_ID` → task UUID → project path → git remote.

## Owner review

When a PR is created, the task owner receives a notification via `ttal send` containing the PR URL, worktree path, and review instructions.

The owner reviews using the `sp-review-against-plan` skill:
- **In-scope + done** ✓ — no action needed
- **In-scope + undone** 🔴 — **always blocking**, sends feedback to worker
- **Cosmetic + no value** ⚪ — not mentioned

LGTM advances the pipeline (`ttal go <uuid>`), which spawns the specialized reviewer pass. NEED_WORK sends blockers directly to the worker via `ttal send`.

## Specialized review

When the owner fires `ttal go <uuid>`, pr-review-lead spawns 6 specialized agents:

1. **Code reviewer** — general quality, style, best practices
2. **Silent failure hunter** — catch blocks, error suppression, missing error handling
3. **Type design analyzer** — type encapsulation, invariant enforcement
4. **Code simplifier** — unnecessary complexity, opportunities to simplify
5. **Comment analyzer** — comment accuracy, staleness, technical debt
6. **Test analyzer** — test coverage gaps, missing edge cases

Each reviewer posts its findings as PR comments with confidence scores.

## Triage flow

The reviewer is advisory only — they post a verdict but never merge:

1. **Reviewer** posts `VERDICT: LGTM` or `VERDICT: NEEDS_WORK`
2. **Worker** triages the review — even with LGTM, there may be non-blocking issues
3. **Worker** fixes actionable issues, pushes updates
4. **Worker** posts a triage summary via `ttal comment add`

## Merging

```bash
# Squash-merge the PR (branch deleted automatically)
ttal go <uuid>
```

After merge, `ttal go <uuid>` drops a cleanup request file to `~/.ttal/cleanup/`. The daemon picks it up and handles the full lifecycle: close tmux session, remove worktree, mark task done.

## Managing comments

```bash
# Add a comment
ttal comment add "Fixed the auth timeout. Ready for re-review."

# List comments
ttal comment list
```

## Approving from Telegram

You can monitor the entire PR lifecycle from Telegram:
- Get notified when PRs are created (owner receives `ttal send`)
- Read review verdicts
- Send merge approval

The worker handles the actual merge command — you just give the green light from your phone.
