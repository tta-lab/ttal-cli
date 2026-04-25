---
title: Messaging
description: Agent-to-human and agent-to-agent messaging via explicit ttal send
---

ttal provides bidirectional messaging between humans and agents via Telegram/Matrix, plus agent-to-agent communication via tmux.

## Outbound: agent → human

`ttal send --to <alias> "message"` is the only way to reach a human (e.g. `ttal send --to neil`). JSONL session output is private workspace — nothing auto-forwards.

```bash
# One-liner
ttal send --to neil "done, PR ready"

# Piped stdin (auto-detected)
echo "check complete" | ttal send --to neil

# Multiline via heredoc
cat <<'ENDBASH' | ttal send --to neil
## Status
Review complete — 2 findings.
ENDBASH
```

Long content: write to flicknote first, then send a one-line pointer:
```bash
flicknote add "detailed findings..." --project notes
ttal send --to neil "wrote note: flicknote abc12345"
```

## Inbound: human → agent

The daemon polls each agent's Telegram bot and Matrix rooms for incoming messages and delivers them to the agent's Claude Code session.

### Setup

1. Create a Telegram bot via [@BotFather](https://t.me/BotFather)
2. Add the bot token to `~/.config/ttal/.env`:

```bash
KESTREL_BOT_TOKEN=123456:ABC...
```

Convention: `{UPPER_AGENT_NAME}_BOT_TOKEN`. The daemon reads this at startup — no config.toml entry needed.

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

Inbound messages arrive in the agent's session with prefixes:

```
[telegram from:neil]
Can you check the deployment?

<i>--- Reply with: ttal send --to neil "your message"</i>
```

### Interactive questions

When Claude Code uses `AskUserQuestion`, ttal displays it in Telegram with inline buttons for each option, plus a "Type answer" button for custom input. You can respond to agent questions right from your phone.

## Agent-to-agent messaging

Agents communicate with each other directly via `ttal send`:

```bash
# Send a message to another agent
ttal send --to athena "Can you research the auth library options?"

# Piped stdin (auto-detected)
echo "Task complete" | ttal send --to kestrel
```

When sent from an agent session (where `TTAL_AGENT_NAME` is set), the recipient sees attribution:

```
[agent from:inke]
The design plan is ready for review.

<i>--- Reply with: ttal send --to inke "your message"</i>
```

## CC control commands

From Telegram, you can send bot commands to control the agent's Claude Code session:

- `/new` — start a new conversation
- `/compact` — compact the context
- `/wait` — pause the agent
