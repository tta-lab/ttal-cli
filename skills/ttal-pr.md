---
name: ttal-pr
description: Manage pull requests from your worker session.
---

# ttal pr

Manage pull requests from your worker session. Context is auto-resolved from the worktree path — no flags needed.

## Create a PR

```bash
ttal pr create "feat: add user authentication"
echo "Fixes #42" | ttal pr create "fix: timeout bug"

# Or with a heredoc for multi-line body:
cat <<'BODY' | ttal pr create "feat: major refactor"
Description spanning multiple lines.
BODY
```

Creates a PR using your task's branch. The PR index is stored in the task automatically.

## Modify a PR

```bash
# Update title
ttal pr modify --title "updated title"

# Update body (stdin)
echo "updated description" | ttal pr modify

# Override PR number for non-worktree use
echo "new body" | ttal pr modify --pr-id 42
```

## Merge a PR (squash)

Merging is handled automatically by `ttal go <uuid>` when `+lgtm` is set on the task.
The daemon squash-merges the PR and requests cleanup — no separate merge command needed.

## Comment on a task

```bash
ttal comment add "LGTM — no critical issues"
ttal comment list
```
