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
ttal task get                           # auto-resolves from $TTAL_JOB_ID (worker) or $TTAL_AGENT_NAME (agent)
ttal task find <keyword>               # search pending tasks (OR, case-insensitive)
ttal task find <keyword1> <keyword2>   # multiple keywords use OR logic
ttal task find <keyword> --completed   # search completed tasks
```

`ttal task get` inlines markdown files from annotations matching `Plan:`, `Design:`, `Doc:`, `Reference:`, or `File:` patterns — useful for feeding full context to agents.

## Advance tasks through the pipeline

Move a task to the next pipeline stage (routes to agent or spawns worker based on config).

```bash
ttal go <uuid>                         # advance to next pipeline stage
```

**When to use:**
- `ttal go` — moves the task through the configured pipeline: routes to the right agent for design/review stages, or spawns a worker for implementation stages. Gate type (auto/human) is determined by the pipeline config.
