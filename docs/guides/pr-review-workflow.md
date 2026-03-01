---
title: PR Review Workflow
description: Autonomous PR creation, review, and merge pipeline
---

ttal supports an autonomous PR workflow where agents implement features, create PRs, get automated reviews, triage feedback, and merge — all auditable on the PR page.

## The pipeline

```
Worker implements task
    ↓
ttal pr create "feat: add auth"
    ↓
6 specialized review agents post comments
    ↓
Worker triages review feedback
    ↓
ttal pr merge (or approve from Telegram)
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

## Automated review

When a PR is created, specialized review agents analyze the code and post comments directly on the PR:

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
4. **Worker** posts a triage summary via `ttal pr comment create`

## Merging

```bash
# Squash-merge the PR (deletes branch by default)
ttal pr merge

# Keep the branch after merge
ttal pr merge --keep-branch
```

After merge, `ttal pr merge` drops a cleanup request file to `~/.ttal/cleanup/`. The daemon picks it up and handles the full lifecycle: close tmux session, remove worktree, mark task done.

## Managing PR comments

```bash
# Add a comment
ttal pr comment create "Fixed the auth timeout. Ready for re-review."

# List comments
ttal pr comment list
```

## Approving from Telegram

You can monitor the entire PR lifecycle from Telegram:
- Get notified when PRs are created
- Read review verdicts
- Send merge approval

The worker handles the actual merge command — you just give the green light from your phone.

## What makes this different

Most coding agent projects stop at "agent wrote some code." ttal's PR workflow means:
- Every implementation is a PR with a structured description
- Every PR gets automated review from 6 specialized agents
- All review findings and fixes are documented on the PR page
- The entire journey from task → implementation → review → merge is auditable
