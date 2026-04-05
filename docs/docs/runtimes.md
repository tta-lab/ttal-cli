---
title: Runtimes
description: Multi-runtime support — Claude Code, Codex CLI, and more
---

ttal is runtime-agnostic. It manages agent sessions via tmux and doesn't care what coding CLI runs inside.

## Supported runtimes

| Runtime | Status | Description |
|---------|--------|-------------|
| **Claude Code** | Stable | Anthropic's CLI. Fully supported, battle-tested. |
| **Codex CLI** | Experimental | OpenAI's coding CLI. Adapter exists but not battle-tested. |
| **Lenos** | Experimental | Lightweight worker runtime via `lenos` binary. |

## Configuration

### Team-level default

Set the default runtime for all agents and workers in a team:

```toml
[teams.default]
default_runtime = "claude-code"
```

### Per-agent override

Set `default_runtime` in the agent's `CLAUDE.md` frontmatter to override the team default for that agent:

```yaml
---
name: coder
default_runtime: lenos
---
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
Coding harness (optional — e.g., oh-my-claudecode)
    ↓
Runtime (Claude Code / Codex CLI / Lenos)
```

ttal operates at the coordination layer. Coding harnesses like oh-my-claudecode operate at the coding layer — they make individual agents code better within a session. The two layers are complementary, not competitive.
