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
├── roles.toml      — manager-plane per-role prompt templates (instructional text, no skills)
├── pipelines.toml  — pipeline stage definitions with gates, reviewers, and skills
├── projects.toml   — project registry
└── .env            — secrets (bot tokens, API keys)
```

**Boundary:** `prompts.toml` holds worker-plane templates (spawned workers and reviewers).
`roles.toml` holds manager-plane per-role templates (long-running agents for task routing).
Skills are configured per-stage in `pipelines.toml`, not per-role in `roles.toml`.

## Basic structure

```toml
shell = "zsh"           # Shell used in tmux sessions (zsh or fish)
default_team = "default"

[teams.default]
data_dir = "~/.ttal"
taskrc = "~/.taskrc"
chat_id = "123456789"           # Global Telegram chat ID
team_path = "~/clawd"           # Root path for agent workspaces
default_runtime = "claude-code"  # Default runtime for agents and workers (claude-code, codex, lenos)
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

## Team fields

| Field | Type | Description |
|-------|------|-------------|
| `data_dir` | string | Data directory (default: `~/.ttal`) |
| `taskrc` | string | Path to taskwarrior config |
| `chat_id` | string | Telegram chat ID |
| `notification_token_env` | string | Override env var for notification bot token (default: `DEFAULT_NOTIFICATION_BOT_TOKEN`) |
| `default_runtime` | string | Default runtime for agents and workers (`claude-code`, `codex`, `lenos`) |
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

## pipelines.toml fields

Per-pipeline stage config lives in `~/.config/ttal/pipelines.toml`:

| Field | Type | Description |
|-------|------|-------------|
| `name` | string | Stage name (alphanumeric + underscore, used as taskwarrior tag) |
| `assignee` | string | Role from roles.toml (e.g. "designer") or "coder" for worker stages |
| `gate` | string | "human" (requires Telegram approval) or "auto" |
| `reviewer` | string | Reviewer agent name (optional) |
| `skills` | string[] | Skill names loaded via `skill get` at stage entry (optional) |

### Stage Skills

Stages can declare skills that are inlined into the agent's context at task routing time:

```toml
[[standard.stages]]
name = "Plan"
assignee = "designer"
gate = "human"
skills = ["sp-planning", "flicknote"]
```

Skills listed here are passed to the agent at stage entry via `skill get`.
This ties skills to what the agent is doing (stage), not who they are (role) — the same agent can use different skills at different stages.

## prompts.toml fields

Worker-plane prompt templates live in `~/.config/ttal/prompts.toml`:

```toml
execute = "Implement this task. The plan is in your task context.\nFollow each task in order: read the plan, make changes, verify, commit."
triage = "Execute `skill get triage`\n\nPR review posted.{{review-file}} Read it and fix issues."
review = "Review PR #{{pr-number}}."
re_review = "Re-review the fixes: {{review-scope}}"
```

| Key | Used by | Template variables |
|-----|---------|-------------------|
| `execute` | `ttal go` | `{{task-id}}` |
| `triage` | PR review → coder | `{{review-file}}` |
| `review` | Reviewer initial prompt | `{{pr-number}}`, `{{pr-title}}`, `{{owner}}`, `{{repo}}`, `{{branch}}` |
| `re_review` | Re-review after fixes | `{{review-scope}}`, `{{coder-comment}}` |

See [Prompts](./prompts.md) for full documentation and examples.

## Notification bot token

System notifications (daemon ready, CI status, worker lifecycle) use a dedicated notification bot token, separate from agent bot tokens.

**Convention:** `DEFAULT_NOTIFICATION_BOT_TOKEN` in `~/.config/ttal/.env`

```env
DEFAULT_NOTIFICATION_BOT_TOKEN=123456:ABC-xyz
```

Override the env var name with `notification_token_env`:

```toml
[teams.default]
notification_token_env = "MY_CUSTOM_BOT_TOKEN"
```
## Environment variables

| Variable | Description |
|----------|-------------|
| `TTAL_AGENT_NAME` | Set automatically in agent sessions — identifies the current agent |
| `TTAL_JOB_ID` | Set automatically in worker sessions — task UUID prefix |
| `FORGEJO_URL` | Forgejo instance URL (for PR commands) |
| `FORGEJO_TOKEN` | Forgejo API token |

</div>
