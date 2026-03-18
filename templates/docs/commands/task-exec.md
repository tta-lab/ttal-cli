---
name: task-exec
description: "Spawn a worker to execute a task"
argument-hint: "<task-uuid or description>"
claude-code:
  allowed-tools:
    - Bash
---

# Task Execute

Spawn a worker to execute a task.

## Usage

```
/task-execute <task-uuid>
```

## Workflow

1. Get the task UUID — from `task <id> export | jq -r '.[0].uuid'` or `ttal task find <keyword>`
2. Run: `ttal task execute <uuid>`

## Example

```
/task-execute 20f5d972
```

This spawns a worker in a new tmux session + git worktree to implement the task.
