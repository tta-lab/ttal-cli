---
name: git-omz
description: Use git plugin abbreviations (similar to oh-my-zsh) for git operations in Fish. Always use grt (git root) before git add/commit in monorepos to ensure you're in the root directory.
---

# Git Aliases (Fish Abbreviations)

Use these abbreviations for common git operations. They are provided by the `jhillyerd/plugin-git` plugin for Fish.

## Critical Pattern for Monorepos

Always jump to git root before add/commit:
```bash
grt && git add <files> && git commit -m "message"
```

This ensures you're in the repo root, avoiding path issues.

## Common Aliases

### Navigation
- `grt` - cd to git root directory

### Status & Diff
- `gst` - git status
- `gd` - git diff
- `gdca` - git diff --cached

### Staging & Committing
- `ga` - git add
- `gaa` - git add --all
- `gc` - git commit
- `gcm "msg"` - git commit -m "msg"
- `gc!` - git commit --amend

### Branches
- `gb` - git branch
- `gco` - git checkout
- `gcb` - git checkout -b
- `gm` - git merge

### Remote
- `gr` - git remote
- `gf` - git fetch
- `gl` - git pull
- `gp` - git push
- `gpf` - git push --force-with-lease

### Logs
- `glog` - git log --oneline --decorate --color --graph
- `gloga` - git log --oneline --decorate --color --graph --all
- `glo` - git log --oneline --decorate --color

## Example Workflow

```bash
# In monorepo subdirectory, commit changes
grt && ga workers/bot && gcm "fix(bot): update handler"

# Push to remote
gp
```
