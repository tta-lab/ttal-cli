# CC→Telegram Bridge via Stop Hook

**Date:** 2026-02-18
**Status:** Draft

## Overview

Replace manual `ttal send --from` calls with an automatic bridge. CC's Stop hook fires at every turn end, calling `ttal bridge` which reads the last assistant text from the session JSONL and sends it to Telegram via the daemon socket.

Agents write normal text. The bridge handles routing. Zero agent awareness required.

```
CC turn ends → Stop hook → ttal bridge (reads JSONL) → daemon socket → Telegram
```

## Changes

### 1. Config Migration

`~/.ttal/daemon.json` → `~/.config/ttal/config.toml`

TOML format, same structure. New dependency: `BurntSushi/toml`.

```toml
zellij_session = "ttal-team"
chat_id = "123"
lifecycle_agent = "kestrel"

[agents.kestrel]
bot_token = "..."

[agents.athena]
bot_token = "..."
```

**Files changed:**
- `internal/daemon/config.go` — `ConfigPath()` returns `~/.config/ttal/config.toml`, `LoadConfig()` uses TOML decode, `WriteTemplate()` writes TOML, `SyncTokens()` simplified (no `json.RawMessage` hack)
- PID file and socket remain in `~/.ttal/` (runtime state, not config)

### 2. New Command: `ttal bridge`

Called by CC's Stop hook, not by humans. Hidden from help output.

**Files:**
- `cmd/bridge.go` — cobra command
- `internal/bridge/bridge.go` — core logic
- `internal/bridge/jsonl.go` — JSONL parsing structs

**Flow:**

1. Read JSON from stdin (CC provides session metadata)
2. If `stop_hook_active` is true → exit 0 (loop prevention)
3. Open ttal database, query agent where `path == cwd`
4. If no matching agent → exit 0 (not a managed session)
5. Read last ~20 lines of `transcript_path`
6. Scan backwards for last `type: "assistant"` entry with a `text` content block
7. Extract text, trim whitespace. If empty → exit 0
8. Send to daemon via socket: `SendRequest{From: agentName, Message: text}`
9. Daemon's existing `handleFrom` routes it to Telegram
10. Silently swallow any daemon errors (hook should not produce output)

**Stdin schema (from CC Stop hook):**

```json
{
  "session_id": "uuid",
  "transcript_path": "/path/to/session.jsonl",
  "cwd": "/path/to/agent/workspace",
  "stop_hook_active": false
}
```

**JSONL parsing structs:**

```go
type jsonlEntry struct {
    Type    string          `json:"type"`
    Message json.RawMessage `json:"message,omitempty"`
}

type assistantMessage struct {
    Content []contentBlock `json:"content"`
}

type contentBlock struct {
    Type string `json:"type"`
    Text string `json:"text,omitempty"`
}
```

**Filtering:** Only skip empty/whitespace text. No length thresholds, no narration pattern detection. The "last text in a completed turn" strategy naturally filters mid-turn narration.

**Separation of concerns:** The bridge only needs the ttal database (to resolve `cwd` → agent name) and the daemon socket (to send). It does not read `config.toml` or check bot tokens — that's the daemon's job.

### 3. `ttal send` Simplification

Remove the solo `--from` code path. With the bridge handling agent→Telegram, `ttal send --from <agent> "msg"` (without `--to`) is redundant from the CLI.

**Before:**
| Flags | Route |
|---|---|
| `--from kestrel` | Agent → Telegram |
| `--to kestrel` | Deliver to agent (zellij) |
| `--from yuki --to kestrel` | Agent → agent (zellij) |

**After:**
| Flags | Route |
|---|---|
| `--to kestrel` | Deliver to agent (zellij) |
| `--from yuki --to kestrel` | Agent → agent (zellij) |
| `--from kestrel` (no --to) | **Error** |

**Note:** The daemon's `handleFrom` is NOT removed — the bridge still calls it via socket. Only the CLI path is removed from `cmd/send.go`.

### 4. Hook Installation (Manual)

Add to `~/.claude/settings.json`:

```json
{
  "hooks": {
    "Stop": [
      {
        "hooks": [
          {
            "type": "command",
            "command": "ttal bridge",
            "timeout": 5
          }
        ]
      }
    ]
  }
}
```

## Architecture

```
┌──────────────┐    stdin (existing)    ┌──────────────┐
│  Telegram     │──────────────────────▶│  Claude Code  │
│  Bot/Daemon   │                       │  (agent)      │
│               │◀──────────────────────│               │
└──────────────┘    Stop hook fires     └──────────────┘
       ▲              at turn end              │
       │                                       ▼
       │                              transcript.jsonl
       │                                       │
       │         ┌─────────────┐               │
       └─────────│ ttal bridge │◀──────────────┘
          socket │ (reads last │  reads last 20 lines
                 │  text, sends│
                 │  to daemon) │
                 └─────────────┘
```

**Data flow:**
1. Telegram → daemon poller → zellij write-chars → CC (inbound, unchanged)
2. CC turn ends → Stop hook → `ttal bridge` → daemon socket → `handleFrom` → Telegram API (outbound, new)

## Edge Cases

- **Non-agent CC sessions:** `cwd` won't match any agent path → silent exit
- **Agent without bot token:** Daemon's `handleFrom` returns error, bridge swallows it silently
- **Loop prevention:** Check `stop_hook_active` first. Bridge produces no stdout so loops shouldn't occur, but check anyway as free insurance.
- **Telegram message limit:** 4096 chars. Handle in daemon (truncate or chunk) — not bridge's concern.
- **Hook timeout:** 5 seconds. JSONL read is fast, socket send is fast. Telegram API call happens in daemon.
- **Subagent sessions:** Run in same cwd but cwd-based lookup still returns the parent agent. The subagent's text may be bridged — acceptable since it's work done on behalf of the agent.

## Dependencies

- New: `github.com/BurntSushi/toml`
- Existing: `github.com/go-telegram/bot` (unchanged)

## Migration Path

1. Deploy config migration (JSON → TOML)
2. Deploy `ttal bridge` command
3. Install global Stop hook
4. Test with one agent
5. Remove `ttal send --from` instructions from agent CLAUDE.md files
6. Remove solo `--from` CLI path
