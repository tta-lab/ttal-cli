---
title: Runtimes
description: Multi-runtime support — Claude Code, OpenCode, Codex CLI, and more
---

ttal is runtime-agnostic. It manages agent sessions via tmux and doesn't care what coding CLI runs inside.

## Supported runtimes

| Runtime | Status | Description |
|---------|--------|-------------|
| **Claude Code** | Stable | Anthropic's CLI. Fully supported, battle-tested. |
| **OpenCode** | Experimental | Open-source coding agent. Adapter exists but not battle-tested. |
| **Codex CLI** | Experimental | OpenAI's coding CLI. Adapter exists but not battle-tested. |

## Configuration

### Team-level default

Set the default runtime for all agents and workers in a team:

```toml
[teams.default]
agent_runtime = "claude-code"
worker_runtime = "claude-code"
```

### Per-task override via tags

Task tags can trigger runtime overrides:

- `+oc` → OpenCode
- `+cx` → Codex CLI

```bash
task add "Implement feature X" +oc    # Worker will use OpenCode
```

## How the adapter works

Each runtime adapter knows how to:

1. **Launch** — the command to start the coding session in tmux
2. **Deliver** — how to send prompts and messages to the session
3. **Read** — how to tail session output (JSONL for Claude Code)

The adapter registry maps runtime names to their implementations. Claude Code is the reference implementation — other adapters follow the same interface.

## Architecture

```
Human (Telegram)
    ↓
ttal (coordination layer)
    ↓
Coding harness (optional — e.g., oh-my-opencode)
    ↓
Runtime (Claude Code / OpenCode / Codex CLI)
```

ttal operates at the coordination layer. Coding harnesses like oh-my-opencode or oh-my-claudecode operate at the coding layer — they make individual agents code better within a session. The two layers are complementary, not competitive.
