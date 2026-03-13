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

```bash
ttal pr merge
ttal pr merge --keep-branch
```

Squash-merges the PR. Fails with a clear error if checks are failing or there are conflicts.

## Comment on a PR

```bash
ttal pr comment create "LGTM — no critical issues"
ttal pr comment list
```
