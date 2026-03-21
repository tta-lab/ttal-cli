---
name: ttal-pr
description: Manage pull requests from your worker session.
---

# ttal pr

Manage pull requests from your worker session. Context is auto-resolved from TTAL_JOB_ID — no flags needed.

## Create a PR

```bash
ttal pr create "feat: add user authentication"
ttal pr create "fix: timeout bug" --body "Fixes #42"
```

Creates a PR using your task's branch. The PR index is stored in the task automatically.

## Modify a PR

```bash
ttal pr modify --title "updated title"
ttal pr modify --body "updated description"
```

## Merge a PR (squash)

Merging is handled automatically by `ttal go <uuid>` when `+lgtm` is set on the task.
The daemon squash-merges the PR and requests cleanup — no separate merge command needed.

## Comment on a task

```bash
ttal comment add "LGTM — no critical issues"
ttal comment list
```
