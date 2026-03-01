---
title: Configuration
description: Config file reference for ttal
---

ttal uses a TOML config file at `~/.config/ttal/config.toml`.

## Basic structure

```toml
shell = "zsh"           # Shell used in tmux sessions (zsh or fish)
default_team = "default"

[teams.default]
data_dir = "~/.ttal"
taskrc = "~/.taskrc"
chat_id = "123456789"           # Global Telegram chat ID
lifecycle_agent = "kestrel"     # Agent that handles task lifecycle events
agent_runtime = "claude-code"   # Default runtime for agents
worker_runtime = "claude-code"  # Default runtime for workers

# Agent-specific config
[teams.default.agents.kestrel]
bot_token = "123456:ABC..."     # Telegram bot token
```

## Global fields

| Field | Type | Description |
|-------|------|-------------|
| `shell` | string | Shell for tmux sessions: `zsh` or `fish` |
| `default_team` | string | Which team to use by default |

## Team fields

Each team lives under `[teams.<name>]`:

| Field | Type | Description |
|-------|------|-------------|
| `data_dir` | string | Data directory (default: `~/.ttal`) |
| `taskrc` | string | Path to taskwarrior config |
| `chat_id` | string | Default Telegram chat ID for this team |
| `lifecycle_agent` | string | Agent that receives lifecycle events |
| `agent_runtime` | string | Default runtime: `claude-code`, `opencode`, `codex`, `openclaw` |
| `worker_runtime` | string | Default runtime for spawned workers |
| `design_agent` | string | Agent name for `ttal task design` |
| `research_agent` | string | Agent name for `ttal task research` |
| `test_agent` | string | Agent name for `ttal task test` |
| `voice_language` | string | ISO 639-1 language code for STT, or `auto` |
| `voice_vocabulary` | list | Custom vocabulary words to improve Whisper accuracy |
| `gateway_url` | string | Gateway URL for webhook-based runtimes |

## Agent fields

Each agent lives under `[teams.<team>.agents.<name>]`:

| Field | Type | Description |
|-------|------|-------------|
| `bot_token` | string | Telegram bot token for this agent |
| `chat_id` | string | Override team-level chat ID |
| `runtime` | string | Override team-level runtime |
| `model` | string | Preferred model: `haiku`, `sonnet`, `opus` |
| `port` | integer | Port for gateway-based runtimes |

## Multi-team configuration

Run separate teams with different taskwarrior instances and runtimes:

```toml
default_team = "personal"

[teams.personal]
data_dir = "~/.ttal"
taskrc = "~/.taskrc"
chat_id = "123456"
lifecycle_agent = "kestrel"

[teams.personal.agents.kestrel]
bot_token = "bot123:ABC"

[teams.work]
data_dir = "~/.ttal-work"
taskrc = "~/.task-work/taskrc"
chat_id = "789012"
lifecycle_agent = "atlas"

[teams.work.agents.atlas]
bot_token = "bot456:DEF"
```

Switch teams with the `TTAL_TEAM` environment variable:

```bash
TTAL_TEAM=work ttal today list
```

## Database

The SQLite database stores agent and project records. Default location: `~/.ttal/ttal.db` (inside `data_dir`).

Override with a flag:

```bash
ttal --db=/custom/path/ttal.db project list
```

## Environment variables

| Variable | Description |
|----------|-------------|
| `TTAL_TEAM` | Override the default team |
| `TTAL_AGENT_NAME` | Set automatically in agent sessions — identifies the current agent |
| `TTAL_JOB_ID` | Set automatically in worker sessions — task UUID prefix |
| `FORGEJO_URL` | Forgejo instance URL (for PR commands) |
| `FORGEJO_TOKEN` | Forgejo API token |
