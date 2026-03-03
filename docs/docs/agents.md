---
title: Agents
description: Agent management and identity in ttal
---

Agents in ttal are persistent identities defined by their workspace directories. Each agent has a directory containing a `CLAUDE.md` file with optional frontmatter for metadata like voice, emoji, and description.

## How agents are discovered

Agents are discovered from the filesystem — any subdirectory of `team_path` (configured in `config.toml`) that contains a `CLAUDE.md` file is treated as an agent. The directory name is the agent name.

```
~/ttal-workspace/
  kestrel/CLAUDE.md    → agent "kestrel"
  athena/CLAUDE.md     → agent "athena"
  docs/                → not an agent (no CLAUDE.md)
```

## CLAUDE.md frontmatter

Agent metadata lives in YAML frontmatter at the top of `CLAUDE.md`:

```markdown
---
voice: af_heart
emoji: 🦅
description: Worker lifecycle management
---
# Kestrel

Your agent instructions here.
```

All frontmatter fields are optional:

| Field | Description | Example |
|-------|-------------|---------|
| `voice` | Kokoro TTS voice ID | `af_heart`, `af_sky` |
| `emoji` | Display emoji | `🦅`, `🐱` |
| `description` | Short role summary | `Task orchestration and planning` |

## Adding agents

```bash
# Create an agent directory with CLAUDE.md
ttal agent add kestrel --emoji 🦅 --description "Worker lifecycle"

# Or manually:
mkdir -p ~/ttal-workspace/kestrel
cat > ~/ttal-workspace/kestrel/CLAUDE.md << 'EOF'
---
emoji: 🦅
description: Worker lifecycle management
---
# Kestrel
EOF
```

## Listing agents

```bash
ttal agent list
```

## Agent info

```bash
ttal agent info kestrel
```

Shows the agent's path, description, voice, and emoji.

## Modifying agents

Update frontmatter fields with `field:value` syntax:

```bash
ttal agent modify kestrel voice:af_heart
ttal agent modify kestrel emoji:🦅 description:'Worker lifecycle'
```

## One bot per agent

Each agent gets its own Telegram bot and its own DM chat. This means you can talk to your researcher about research while your designer designs — like messaging actual team members, not @-routing in a single chat.

Configure bot tokens in `config.toml`:

```toml
[teams.default.agents.kestrel]
bot_token_env = "KESTREL_BOT_TOKEN"
```

Create a bot via [@BotFather](https://t.me/BotFather) on Telegram for each agent.
