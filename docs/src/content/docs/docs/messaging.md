---
title: Messaging
description: Telegram bridge and agent communication
sidebar:
  order: 4
---

ttal provides bidirectional messaging between humans and agents via Telegram, plus agent-to-agent communication via tmux.

## Telegram bridge

The daemon polls each agent's Telegram bot for incoming messages and delivers them to the agent's tmux session.

### Setup

1. Create a Telegram bot via [@BotFather](https://t.me/BotFather)
2. Add the bot token to your config:

```toml
[teams.default.agents.kestrel]
bot_token = "123456:ABC..."
```

3. Start a chat with your bot on Telegram
4. The daemon will begin polling automatically

### Multimodal input

Send anything to your agent via Telegram:

- **Text** — delivered directly to the agent's terminal
- **Voice messages** — transcribed via Whisper, delivered as text
- **Photos and screenshots** — downloaded and delivered with file paths
- **Files** — downloaded to a team-specific directory, path sent to agent
- **Links** — forwarded to the agent in context

The bot handles transcription, file downloads, and delivers everything to your agent automatically.

### Message format

Messages arrive in the agent's terminal with prefixes:

```
[telegram from:neil]
Can you check the deployment?
```

### Interactive questions

When Claude Code uses `AskUserQuestion`, ttal displays it in Telegram with inline buttons for each option, plus a "Type answer" button for custom input. You can respond to agent questions right from your phone.

## Agent-to-agent messaging

Agents communicate with each other directly via `ttal send`:

```bash
# Send a message to another agent
ttal send --to athena "Can you research the auth library options?"

# Read from stdin
echo "Task complete" | ttal send --to kestrel --stdin
```

When sent from an agent session (where `TTAL_AGENT_NAME` is set), the recipient sees attribution:

```
[agent from:inke]
The design plan is ready for review.
```

## Agent to human

Agents don't need to call `ttal send` to reach humans. The daemon's JSONL watcher automatically tails active Claude Code session files and sends assistant text blocks to Telegram. Agents just write normal output — the watcher handles routing.

## CC control commands

From Telegram, you can send bot commands to control the agent's Claude Code session:

- `/new` — start a new conversation
- `/compact` — compact the context
- `/wait` — pause the agent

These commands appear in Telegram's `/` menu automatically.
