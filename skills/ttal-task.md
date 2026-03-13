---
name: ttal-task
description: Create tasks and export rich prompts for piping to agents.
---

# ttal task

Create tasks and export rich prompts for piping to agents.

## Create a task

```bash
# Project is required, validated against ttal project DB
ttal task add --project <alias> "description" --tag <tag> --priority M --annotate "note"

# Tags and annotations are repeatable
ttal task add --project ttal "Fix auth bug" --tag bugfix --tag urgent --priority H \
  --annotate "Stack trace in #general" --annotate "Repo: /Users/neil/Code/..."
```

`ttal task add` validates the project against the ttal project database — use `ttal project list` to see valid aliases. The on-add hook handles `project_path` and `branch` UDAs automatically.

## Search and export tasks

```bash
ttal task get <uuid>                    # export task as rich prompt (inlines referenced docs)
ttal task get                           # uses $TTAL_JOB_ID env var
ttal task find <keyword>               # search pending tasks (OR, case-insensitive)
ttal task find <keyword1> <keyword2>   # multiple keywords use OR logic
ttal task find <keyword> --completed   # search completed tasks
```

`ttal task get` inlines markdown files from annotations matching `Plan:`, `Design:`, `Doc:`, `Reference:`, or `File:` patterns — useful for feeding full context to agents.

## Route tasks

Route tasks to the right agent instead of doing everything yourself.

```bash
ttal task route <uuid> --to <agent>              # route to agent for design/research/brainstorm
ttal task route <uuid> --to <agent> --message "context"  # add context
ttal task execute <uuid>                         # spawn a worker
```

**When to use:**
- `ttal task route` — task needs design, research, or brainstorming
- `ttal task execute` — task has a plan/design doc annotated and is ready to implement. Spawns a worker in its own tmux session + git worktree.
