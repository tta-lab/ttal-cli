# ttal daemon — Bidirectional Agent Communication

> **Implementation note (post-design):** `ttal notify` was replaced by `ttal send --from/--to`
> during implementation. The socket protocol uses `SendRequest{From, To, Message}` instead of
> `NotifyRequest{Agent, Message}`. See `internal/daemon/socket.go` for the current API.
> Routing is direction-aware: `--from` only → Telegram, `--to` only → Zellij,
> both → agent-to-agent via Zellij with `[agent from:name]` attribution.

> **Implementation note (2026-03-14):** The daemon was migrated from raw
> newline-delimited JSON over unix socket to HTTP over unix socket.
> Routes: POST /send, GET /status, POST /status/update, POST /task/complete, GET /health.
> Client code uses `http.Client` with a unix socket transport.

## Overview

A local daemon embedded in `ttal` that provides bidirectional communication between CC agents running in zellij and humans on Telegram. It replaces the current OpenClaw-based notification and the standalone poll-completion launchd service with a single, unified process.

## Architecture

```
ttal notify ──→ unix socket ──→ daemon ──→ Telegram Bot API
                                  ↑
                               getUpdates (long poll)
                                  ↓
                          zellij write-chars ──→ CC in tab

                        + worker completion poll (every 60s)
```

Single process, managed by launchd. Four concurrent responsibilities:

1. **Unix socket listener** — handles `ttal notify` requests
2. **Telegram poller** — one goroutine per bot token, long-polls `getUpdates`
3. **Worker completion poller** — checks merged PRs every 60s, auto-completes tasks
4. **Signal handler** — graceful shutdown on SIGINT/SIGTERM

## Config

`~/.ttal/daemon.json`:

```json
{
  "agents": {
    "kestrel": {
      "telegram": {
        "bot_token": "123:ABC...",
        "chat_id": "845849177"
      },
      "zellij": {
        "session": "cclaw",
        "tab": "kestrel"
      }
    }
  }
}
```

Each agent maps to one bot token, one chat_id, one zellij target. One bot per chat — no group routing.

## Socket Protocol

Unix socket at `~/.ttal/daemon.sock`. JSON-over-newline.

Request:
```json
{"agent": "kestrel", "message": "PR #42 ready for review"}
```

Response:
```json
{"ok": true}
```

Error:
```json
{"ok": false, "error": "unknown agent: foo"}
```

## Inbound Message Format

Delivered to zellij pane via `write-chars` + Enter:

```
[telegram from:neil]
Can you also handle the edge case?

To reply: ttal notify --agent kestrel "your reply"
```

Full reply instructions included every time so agents always know how to respond.

## CLI Commands

### New commands

```
ttal daemon              # Run daemon foreground (what launchd calls)
ttal daemon install      # Install launchd plist + create config template
ttal daemon uninstall    # Remove launchd plist, socket, pid file
ttal daemon status       # Check if running (pid file + socket check)
ttal notify --agent <name> "message"
ttal notify --agent <name> --stdin   # read from stdin
```

### Removed commands

```
ttal worker poll         # Replaced by daemon's built-in completion poller
```

### Modified commands

```
ttal worker install      # Only installs taskwarrior hook (no poll plist)
ttal worker uninstall    # Only removes taskwarrior hook (no poll plist)
```

## Hook Integration

`notifyAgentWith()` in hook.go changes to connect to daemon socket instead of spawning openclaw or calling zellij directly. The hook becomes a thin socket client.

Falls back to direct zellij delivery if daemon socket is unavailable (daemon not running).

## Package Structure

```
internal/daemon/
  ├── daemon.go        # Main loop: socket + pollers + shutdown
  ├── config.go        # Load/validate ~/.ttal/daemon.json
  ├── socket.go        # Unix socket server + client
  ├── telegram.go      # Bot API: sendMessage + getUpdates
  ├── deliver.go       # Format message + zellij write-chars
  └── launchd.go       # Install/uninstall plist

cmd/
  ├── daemon.go        # daemon, daemon install/uninstall/status
  └── notify.go        # ttal notify
```

## Dependencies

- `go-telegram-bot-api` for Telegram Bot API interaction

## Error Handling

- Telegram poll failure: log, exponential backoff, retry (don't crash)
- Zellij delivery failure (session not found): log warning, skip
- Socket client timeout: 5s
- Invalid agent name: return error to caller
- Daemon already running: check pid file, refuse to start

## launchd Plist

`~/Library/LaunchAgents/io.guion.ttal.daemon.plist`

- `RunAtLoad: true`
- `KeepAlive: true`
- Logs to `~/.ttal/daemon.log`
- Replaces `io.guion.ttal.poll-completion` plist

## Migration

1. `ttal daemon install` removes old `io.guion.ttal.poll-completion` plist if present
2. `ttal worker install` no longer creates a poll plist
3. Hook config (`~/.task/hooks/config.json`) still used by hook for direct delivery fallback
