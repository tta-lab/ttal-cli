---
title: Configuration
description: Config file reference for ttal
---

<div v-pre>

ttal uses a TOML config file at `~/.config/ttal/config.toml`.

## Config file layout

All ttal configuration lives in `~/.config/ttal/`:

```
~/.config/ttal/
├── config.toml     — structure + settings (teams, sync, flicknote, voice)
├── prompts.toml    — worker-plane prompt templates (execute, review, triage, re_review)
├── roles.toml      — manager-plane per-role prompts + heartbeat config
├── projects.toml   — project registry
└── .env            — secrets (bot tokens, API keys)
```

**Boundary:** `prompts.toml` holds worker-plane templates (spawned workers and reviewers).
`roles.toml` holds manager-plane per-role templates (long-running agents for task routing).

## Basic structure

```toml
shell = "zsh"           # Shell used in tmux sessions (zsh or fish)
default_team = "default"

[teams.default]
data_dir = "~/.ttal"
taskrc = "~/.taskrc"
chat_id = "123456789"           # Global Telegram chat ID
team_path = "~/clawd"           # Root path for agent workspaces
agent_runtime = "claude-code"   # Default runtime for agents
worker_runtime = "claude-code"  # Default runtime for workers
```

### Agent discovery

Agents are discovered automatically from the filesystem. Any subdirectory of `team_path`
that contains a `CLAUDE.md` file is treated as an agent.

```
~/clawd/
├── yuki/CLAUDE.md     → agent "yuki"
├── inke/CLAUDE.md     → agent "inke"
├── athena/CLAUDE.md   → agent "athena"
└── docs/              → NOT an agent (no CLAUDE.md)
```

Agent configuration lives in two places:
- **CLAUDE.md frontmatter** — identity (role, emoji, voice, description)
- **roles.toml** — operational config per role (prompts, heartbeat_interval)

Bot tokens follow the naming convention `{UPPER_NAME}_BOT_TOKEN` in `~/.config/ttal/.env`.

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
| `notification_token_env` | string | Override env var for notification bot token (default: `{UPPER_TEAM}_NOTIFICATION_BOT_TOKEN`) |
| `agent_runtime` | string | Default runtime: `claude-code`, `codex` |
| `worker_runtime` | string | Default runtime for spawned workers |
| `voice_language` | string | ISO 639-1 language code for STT, or `auto` |
| `voice_vocabulary` | list | Custom vocabulary words to improve Whisper accuracy |
| `gateway_url` | string | Gateway URL for webhook-based runtimes |

## CLAUDE.md frontmatter fields

Agent identity is configured in CLAUDE.md frontmatter (in the agent's workspace directory):

```yaml
---
description: Task orchestrator — creates and routes work
emoji: 🐱
role: manager
voice: af_heart
---
```

| Field | Type | Description |
|-------|------|-------------|
| `description` | string | Short role summary |
| `emoji` | string | Display emoji |
| `role` | string | Role key that maps to `[role]` in roles.toml |
| `voice` | string | Kokoro TTS voice ID (e.g. `af_heart`) |

## roles.toml fields

Per-role operational config lives in `~/.config/ttal/roles.toml`:

```toml
[manager]
prompt = "..."
heartbeat_interval = "1h"
heartbeat_prompt = "..."

[designer]
prompt = "..."
```

| Field | Type | Description |
|-------|------|-------------|
| `prompt` | string | Routing prompt template for this role |
| `heartbeat_interval` | string | How often to send the heartbeat prompt (e.g. `"1h"`, `"30m"`) |
| `heartbeat_prompt` | string | Prompt delivered on each heartbeat tick |

## prompts.toml fields

Worker-plane prompt templates live in `~/.config/ttal/prompts.toml`:

```toml
execute = "{{skill:sp-executing-plans}}\nwrite this plan task-by-task."
triage = "{{skill:triage}}\nPR review posted.{{review-file}} Read it and fix issues."
review = "{{skill:pr-review}}\nReview PR #{{pr-number}}."
re_review = "{{skill:pr-review}}\nRe-review the fixes: {{review-scope}}"
```

| Key | Used by | Template variables |
|-----|---------|-------------------|
| `execute` | `ttal go` | `{{task-id}}`, `{{skill:name}}` |
| `triage` | PR review → coder | `{{review-file}}`, `{{skill:name}}` |
| `review` | Reviewer initial prompt | `{{pr-number}}`, `{{pr-title}}`, `{{owner}}`, `{{repo}}`, `{{branch}}`, `{{skill:name}}` |
| `re_review` | Re-review after fixes | `{{review-scope}}`, `{{coder-comment}}`, `{{skill:name}}` |

See [Prompts](./prompts.md) for full documentation and examples.

## Notification bot token

System notifications (daemon ready, CI status, worker lifecycle) use a dedicated notification bot token per team, separate from agent bot tokens.

**Convention:** `{UPPER_TEAM}_NOTIFICATION_BOT_TOKEN` in `~/.config/ttal/.env`

```env
# Default team
DEFAULT_NOTIFICATION_BOT_TOKEN=123456:ABC-xyz

# Work team
WORK_NOTIFICATION_BOT_TOKEN=789012:DEF-uvw
```

Override the env var name per team with `notification_token_env`:

```toml
[teams.default]
notification_token_env = "MY_CUSTOM_BOT_TOKEN"
```

## Multi-team configuration

Run separate teams with different taskwarrior instances and runtimes.
Agents are discovered from the filesystem — no per-agent config blocks needed:

```toml
default_team = "personal"

[teams.personal]
data_dir = "~/.ttal"
taskrc = "~/.taskrc"
chat_id = "123456"
team_path = "~/personal-agents"

[teams.work]
data_dir = "~/.ttal-work"
taskrc = "~/.task-work/taskrc"
chat_id = "789012"
team_path = "~/work-agents"
```

Bot tokens in `~/.config/ttal/.env`:

```env
KESTREL_BOT_TOKEN=bot123:ABC
ATLAS_BOT_TOKEN=bot456:DEF
```

Switch teams with the `TTAL_TEAM` environment variable:

```bash
TTAL_TEAM=work ttal today list
```

## Environment variables

| Variable | Description |
|----------|-------------|
| `TTAL_TEAM` | Override the default team |
| `TTAL_AGENT_NAME` | Set automatically in agent sessions — identifies the current agent |
| `TTAL_JOB_ID` | Set automatically in worker sessions — task UUID prefix |
| `FORGEJO_URL` | Forgejo instance URL (for PR commands) |
| `FORGEJO_TOKEN` | Forgejo API token |

</div>
