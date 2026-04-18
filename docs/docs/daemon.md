---
title: Daemon
description: The ttal daemon — communication hub and service manager
---

The daemon is a long-running process that acts as the communication hub for your agent team. It's managed by launchd on macOS.

## What the daemon does

- **Telegram polling** — polls each agent's Telegram bot for incoming messages
- **Message delivery** — delivers messages to agent tmux sessions via `send-keys`
- **Cleanup watcher** — processes post-merge cleanup requests (close session, remove worktree, mark task done)
- **Task routing** — handles `ttal go <uuid>` by advancing tasks through pipeline stages

## Installation

```bash
# Install launchd plist and create config template
ttal daemon install

# Check status
ttal daemon status

# Remove
ttal daemon uninstall
```

## Running manually

For debugging, run the daemon in the foreground:

```bash
ttal daemon
```

## Logs

```bash
tail -f ~/.ttal/daemon.log
```

## HTTP API

The daemon runs an HTTP server on a Unix socket at `~/.ttal/daemon.sock`.

Routes:
- `POST /send` — deliver messages between agents/humans
- `GET /status?team=X&agent=Y` — query agent context status
- `POST /status/update` — write agent context status
- `POST /task/complete` — notify task completion
- `GET /health` — health check

Debug with curl:
```bash
curl --unix-socket ~/.ttal/daemon.sock http://daemon/health
curl --unix-socket ~/.ttal/daemon.sock http://daemon/status
```

Internal commands (like `ttal send` and task routing) communicate with the daemon via HTTP over the unix socket using a `SendRequest` struct:

```go
type SendRequest struct {
    From    string `json:"from"`
    To      string `json:"to"`
    Message string `json:"message"`
}
```

Direction is inferred from which fields are set:
- `To` only → system/hook message to an agent
- `From` + `To` → agent-to-agent with attribution

## Status files

Agent status information is written to `~/.ttal/status/`. The daemon updates these files as it tracks agent sessions.

## Bot tokens

Bot tokens are stored in `~/.config/ttal/.env` and loaded automatically at startup. See the [setup guide](getting-started.md) for details.
