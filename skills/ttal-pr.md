---
name: ttal-pr
description: Manage pull requests from your worker session.
---

# ttal pr

Manage pull requests from your worker session. Context is auto-resolved from the worktree path — no flags needed.

## Create a PR

```
ttal pr create "feat: add user authentication"
echo "Fixes #42" | ttal pr create "fix: timeout bug"

# Or with a heredoc for multi-line body:
cat <<'BODY' | ttal pr create "feat: major refactor"
Description spanning multiple lines.
BODY
```

Creates a PR using your task's branch. The PR index is stored in the task automatically.

## Modify a PR

```
# Update title
ttal pr modify --title "updated title"

# Update body (stdin)
echo "updated description" | ttal pr modify

# Override PR number for non-worktree use
echo "new body" | ttal pr modify --pr-id 42
```

## View a PR

```
ttal pr view
ttal pr list       # alias for same command
```

Resolves the PR for the current branch from any directory in a git repo. Shows:
- PR number, title, state (open/closed/merged)
- PR URL and source → target branch
- Body preview (first 10 lines)
- CI status summary

No task context needed — works in any ttal project directory.

## View CI failure logs

```
ttal pr log
```

Shows CI check status plus failure details and log tails for any failed jobs.
Use this to diagnose CI failures when `ttal pr view` shows failing checks.

## Merge a PR (squash)

Merging is handled automatically by `ttal go <uuid>` when `+lgtm` is set on the task.
The daemon squash-merges the PR and requests cleanup — no separate merge command needed.
