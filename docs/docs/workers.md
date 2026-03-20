---
title: Workers
description: Worker lifecycle — spawn, work, cleanup
---

Workers are isolated coding sessions. Each worker runs in its own tmux session with a dedicated git worktree, so multiple agents can work on different tasks in the same repository simultaneously.

## Spawning a worker

The primary way to spawn a worker is through task execution:

```bash
ttal task go <uuid>
```

This reads the task's metadata (project path, branch name) and spawns a worker with the right context.

### Manual spawn

For more control:

```bash
ttal worker spawn --name fix-auth --project ~/code/myapp --task <uuid>
```

#### Spawn flags

| Flag | Default | Description |
|------|---------|-------------|
| `--name` | required | Worker name (used for branch and session naming) |
| `--project` | required | Project directory path |
| `--task` | required | Taskwarrior task UUID |
| `--worktree` | `true` | Create git worktree for isolation |
| `--force` | `false` | Force respawn (close existing session) |
| `--yolo` | `true` | Skip Claude Code permission prompts |
| `--brainstorm` | `false` | Use brainstorming skill before implementation |

## What happens on spawn

1. A new git branch is created: `worker/<name>`
2. A git worktree is set up in `~/.ttal/worktrees/<name>`
3. A tmux session is created with the coding runtime (Claude Code by default)
4. The task prompt is sent to the agent, including any inlined plan/research docs from annotations

## Listing workers

```bash
ttal worker list
```

Shows all active worker sessions with their task UUIDs and branches.

## Closing a worker

### Smart close

```bash
ttal worker close <session-name>
```

Smart close checks the PR status:
- If the PR is merged and the worktree is clean → auto-cleanup
- Otherwise → returns `ErrNeedsDecision`, causing the CLI to exit with code 1 for manual handling

### Force close

```bash
ttal worker close <session-name> --force
```

Force close dumps the session state and cleans up regardless of PR status.

### Exit codes

| Code | Meaning |
|------|---------|
| 0 | Cleaned up successfully |
| 1 | Needs manual decision (PR not merged, dirty worktree) |
| 2 | Error (worker not found, script failure) |

## Cleanup flow

After a PR is merged, the typical cleanup flow is:

1. Run `ttal pr merge` from the worker session
2. This drops a cleanup request file to `~/.ttal/cleanup/`
3. The daemon picks it up via fsnotify
4. Daemon runs: close tmux session → remove worktree → mark task done

### Processing pending cleanups

```bash
ttal worker cleanup
```

This processes any pending cleanup request files that the daemon hasn't handled yet.

## Execute a task

Spawn a worker to implement a task:

```bash
ttal task go <uuid>
```

Without `--yes`, shows the project path and prompts for confirmation.
