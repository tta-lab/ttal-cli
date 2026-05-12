---
title: Agents
description: Agent management and identity in ttal
---

Agents in ttal are persistent identities defined by flat `.md` files in the team's workspace directory. Each agent has a `.md` file with YAML frontmatter for metadata like voice, emoji, and description.

## How agents are discovered

Agents are discovered from the filesystem — any `.md` file in `team_path` (configured in `config.toml`) is treated as an agent. The filename (without `.md`) is the agent name.

```
~/ttal-workspace/
  kestrel.md     → agent "kestrel"
  athena.md      → agent "athena"
  README.md      → not an agent (skipped)
  CLAUDE.user.md → not an agent (skipped)
```

## Agent frontmatter

Agent metadata lives in YAML frontmatter at the top of the `.md` file:

```markdown
---
voice: af_heart
emoji: 🦅
description: Worker lifecycle management
role: fixer
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
| `role` | Role key matching `[role]` in `roles.toml` | `manager`, `designer` |
| `color` | Claude Code UI color for visual distinction | `blue`, `cyan`, `green`, `yellow`, `red`, `magenta` |
| `lenos.pair_with` | Default Lenos `narrate` target for agents with a static pair target | `coder` |

Agent `.md` frontmatter is the single source of truth for agent identity and per-agent config.
Operational config (prompts, heartbeat) lives in `~/.config/ttal/roles.toml` per role.

## Adding agents

```
# Create an agent .md file
ttal agent add kestrel --emoji 🦅 --description "Worker lifecycle"

# Or manually:
cat > ~/ttal-workspace/kestrel.md << 'EOF'
---
emoji: 🦅
description: Worker lifecycle management
---
# Kestrel
EOF
```

## Listing agents

```
ttal agent list
```

## Agent info

```
ttal agent info kestrel
```

Shows the agent's path, description, voice, and emoji.

## Modifying agents

Update frontmatter fields with `field:value` syntax:

```
ttal agent modify kestrel emoji:🦅 description:'Worker lifecycle'
```

## One bot per agent

Each agent gets its own Telegram bot and its own DM chat. This means you can talk to your researcher about research while your designer designs — like messaging actual team members, not @-routing in a single chat.

Bot tokens use the naming convention `{UPPER_NAME}_BOT_TOKEN` in `~/.config/ttal/.env`:

```env
KESTREL_BOT_TOKEN=123456:ABC-xyz
ATHENA_BOT_TOKEN=789012:DEF-uvw
```

No configuration in `config.toml` is needed — the convention is the only way.

Create a bot via [@BotFather](https://t.me/BotFather) on Telegram for each agent.

Human Telegram chat IDs use the same env-var convention in `~/.config/ttal/.env`:

```env
NEIL_CHAT_ID=123456789
```
