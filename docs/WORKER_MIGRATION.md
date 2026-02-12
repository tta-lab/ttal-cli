# Worker Migration Guide

Migrate from the Python worker scripts (`~/clawd/scripts/`) to the native Go commands in `ttal worker`.

## Command Mapping

| Python Script | Go Command | Notes |
|--------------|------------|-------|
| `spawn_worker.py` | `ttal worker spawn` | Same flags, UUID-only task IDs |
| `list_workers.py` | `ttal worker list` | Table view with PR status categories |
| `close_worker.py` | `ttal worker close` | Same smart/force modes |
| `poll_worker_completion.py` | `ttal worker poll` | Same behavior, logs to `~/.ttal/` |

## Step-by-Step Migration

### 1. Install ttal

```bash
cd ~/Code/guion-opensource/ttal-cli
make install
```

This installs `ttal` to `~/go/bin/ttal`. Ensure `~/go/bin` is in your `$PATH`.

### 2. Set Environment Variables

Add to your shell config (`~/.config/fish/config.fish` or `~/.zshrc`):

```bash
export FORGEJO_URL=https://git.guion.io
export FORGEJO_TOKEN=<your-forgejo-api-token>
```

These are needed for PR merge checks in `ttal worker poll` and `ttal worker close` (smart mode).

### 3. Verify Taskwarrior UDAs

Ensure these are in `~/.taskrc`:

```
uda.session_name.type=string
uda.session_name.label=Session Name

uda.branch.type=string
uda.branch.label=Branch

uda.project_path.type=string
uda.project_path.label=Project Path

uda.pr_id.type=numeric
uda.pr_id.label=PR ID
```

These should already exist if you were using the Python scripts.

### 4. Migrate launchd Service

Uninstall the Python-based poll service:

```bash
~/clawd/scripts/poll-completion-uninstall.sh
```

Install the Go-based poll service:

```bash
# From the ttal-cli repo
./scripts/poll-install.sh
```

Verify:

```bash
launchctl list | grep ttal.poll
# Should show: - 0 io.guion.ttal.poll-completion
```

### 5. Update Taskwarrior Hooks

If your taskwarrior hooks call the Python scripts directly, update them:

```bash
# Old (Python)
python3 ~/clawd/scripts/spawn_worker.py --name "$name" --project "$path" --task "$uuid"

# New (Go)
ttal worker spawn --name "$name" --project "$path" --task "$uuid"
```

```bash
# Old (Python)
python3 ~/clawd/scripts/close_worker.py "$session_name"

# New (Go)
ttal worker close "$session_name"
```

## Command Comparison

### Spawn

```bash
# Python
python3 ~/clawd/scripts/spawn_worker.py \
  --name fix-auth \
  --project ~/code/myapp \
  --task abc12345-def6-7890-abcd-ef1234567890

# Go
ttal worker spawn \
  --name fix-auth \
  --project ~/code/myapp \
  --task abc12345-def6-7890-abcd-ef1234567890
```

Both support `--brainstorm`, `--force`, `--session`, `--worktree`, `--yolo`.

### List

```bash
# Python
python3 ~/clawd/scripts/list_workers.py

# Go
ttal worker list
```

Output is a tabwriter table with columns: SESSION, STATUS, PR, BRANCH, PROJECT, TASK.

Status values: `RUNNING` (no PR yet), `WITH_PR` (PR not merged), `CLEANUP` (PR merged, needs cleanup).

### Close

```bash
# Python
python3 ~/clawd/scripts/close_worker.py a7f3d2b9
python3 ~/clawd/scripts/close_worker.py a7f3d2b9 --force

# Go
ttal worker close a7f3d2b9
ttal worker close a7f3d2b9 --force
```

Same exit codes: 0 (cleaned), 1 (needs decision), 2 (error).

### Poll

```bash
# Python (called by launchd)
python3 ~/clawd/scripts/poll_worker_completion.py

# Go (called by launchd)
ttal worker poll
```

## What Changed

### Log Locations

| What | Python | Go |
|------|--------|-----|
| Poll logs | `~/.clawd-zellij/poll_completion.log` | `~/.ttal/poll_completion.log` |
| State dumps | `~/.clawd-zellij/dumps/` | `~/.ttal/dumps/` |
| launchd stdout | `~/.clawd-zellij/poll_completion_stdout.log` | `~/.ttal/poll_completion_stdout.log` |
| launchd stderr | `~/.clawd-zellij/poll_completion_stderr.log` | `~/.ttal/poll_completion_stderr.log` |

### Zellij Data Directory

| Python | Go |
|--------|-----|
| `$TMPDIR/moltbot-zellij-data` | `$TMPDIR/ttal-zellij-data` |
| `MOLTBOT_ZELLIJ_DATA_DIR` env var | `TTAL_ZELLIJ_DATA_DIR` env var |

### launchd Service

| Python | Go |
|--------|-----|
| `io.guion.clawd.poll-completion` | `io.guion.ttal.poll-completion` |
| Calls `python3 poll_worker_completion.py` | Calls `ttal worker poll` |

### Forgejo Integration

| Python | Go |
|--------|-----|
| Uses `lib/forgejo_helper.py` (HTTP requests) | Uses Forgejo Go SDK (`forgejo-sdk/v2`) |
| Reads `FORGEJO_URL` from env | Same |
| Reads `FORGEJO_TOKEN` from env | Same |

### No Database Required

Worker commands bypass ttal's SQLite database entirely. They interact only with:
- Taskwarrior (task state, UDAs)
- Zellij (session management)
- Git (worktrees, branches)
- Forgejo API (PR status checks)

## Rollback

If you need to go back to the Python scripts:

```bash
# Uninstall Go poll service
./scripts/poll-uninstall.sh

# Reinstall Python poll service
~/clawd/scripts/poll-completion-install.sh
```

The Python scripts and Go commands can coexist safely since they use different zellij data directories and launchd service names.
