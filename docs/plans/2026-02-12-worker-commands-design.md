# Worker Commands Design

Rewrite of `spawn_worker.py`, `close_worker.py`, and `poll_worker_completion.py` from Python to Go as `ttal worker` subcommands.

## Commands

```
ttal worker spawn --name X --project /path --task <uuid> [--force] [--no-worktree] [--brainstorm] [--no-yolo]
ttal worker close <session-name> [--force]
ttal worker poll
```

## Package Layout

```
cmd/worker.go              # Cobra commands (thin flag parsing)

internal/
  worker/spawn.go          # Worktree setup, orchestrates spawn flow
  worker/close.go          # Smart/force cleanup logic
  worker/poll.go           # PR merge polling + task completion
  taskwarrior/taskwarrior.go  # UDA verify, task query/modify/done
  zellij/zellij.go         # Session create/delete/list, layout gen
  forgejo/forgejo.go       # SDK client, PR merge check, repo info
```

## Key Decisions

- **No ttal database** — worker commands skip DB init via PersistentPreRunE override
- **Gatekeeper stays Python** — process lifecycle management works fine as-is
- **Forgejo Go SDK** (`codeberg.org/mvdkleijn/forgejo-sdk/forgejo/v2`) for PR checks
- **No phase worker support** initially (YAGNI)
- **Taskwarrior via CLI** — shell out to `task` with 5s timeouts
- **Env vars for config**: FORGEJO_URL, FORGEJO_TOKEN, TTAL_ZELLIJ_DATA_DIR

## Config

| Variable | Default | Used by |
|---|---|---|
| FORGEJO_URL | (required for close/poll) | forgejo client |
| FORGEJO_TOKEN | (required for close/poll) | forgejo client |
| TTAL_ZELLIJ_DATA_DIR | $TMPDIR/ttal-zellij-data | zellij sessions |

Logs: `~/.ttal/poll_completion.log`

## spawn flow

1. Validate UUID → load task from taskwarrior
2. Verify required UDAs (session_name, branch, project_path)
3. Setup git worktree (project/.worktrees/<name>, branch worker/<name>)
4. Generate random 8-char session ID, check conflicts (--force to override)
5. Create KDL layout (worker tab + term tab), write task to temp file
6. Launch zellij session with PTY, wait for session creation (10s timeout)
7. Update task with worker metadata UDAs + annotate "Worker: <name>"

## close flow

Exit codes: 0=cleaned, 1=needs decision, 2=error

**Smart mode** (default):
1. Query task by session_name (try completed first, then pending)
2. Check pr_id UDA → forgejo.IsPRMerged()
3. Check worktree clean (git status)
4. If PR merged + clean: delete session, remove worktree, delete branch → exit 0
5. Otherwise: dump state → exit 1

**Force mode** (--force):
1. Dump session state
2. Cleanup regardless → exit 0

## poll flow

Stateless, runs once per invocation (launchd calls every minute):
1. Query active worker tasks (pending + ACTIVE + has session_name)
2. For each: parse repo info from project_path → check PR merged → mark done
3. Cleanup stale /tmp files (claude-task-*.txt, zellij-layout-*.kdl > 24h)
4. Log to ~/.ttal/poll_completion.log
