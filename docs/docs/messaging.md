---
title: Messaging
description: Agent-to-human and agent-to-agent messaging via explicit ttal send
---

ttal provides bidirectional messaging between humans and agents via Telegram/Matrix, plus agent-to-agent communication via tmux.

## Outbound: any recipient

`ttal send --to {recipient}` is the one explicit send path for humans and agents. The recipient can be a human alias, an agent name, or a worker address (`{uuid}:{agent}`). JSONL session output is private workspace — nothing auto-forwards.

```
# Human alias
cat <<'EOF' | ttal send --to {human-alias}
done, PR ready
EOF

# Agent name
cat <<'EOF' | ttal send --to {agent}
Can you research the auth library options?
EOF

# Worker address
cat <<'EOF' | ttal send --to {uuid}:{agent}
## Status
Review complete — 2 findings.
EOF
```

Long content: write to flicknote first, then send a one-line pointer:
```
flicknote add "detailed findings..." --project notes
cat <<'EOF' | ttal send --to {recipient}
wrote note: flicknote abc12345
EOF
```

## Inbound: human → agent

The daemon polls each agent's Telegram bot and Matrix rooms for incoming messages and delivers them to the agent's Claude Code session.

### Setup

1. Create a Telegram bot via [@BotFather](https://t.me/BotFather)
2. Add the bot token to `~/.config/ttal/.env`:

```
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

```text
<- {human-alias}:telegram [14:32:05] Can you check the deployment?

<i>--- Reply with:
cat <<'EOF' | ttal send --to {human-alias}
your message
EOF</i>
```

### Interactive questions

When Claude Code uses `AskUserQuestion`, ttal displays it in Telegram with inline buttons for each option, plus a "Type answer" button for custom input. You can respond to agent questions right from your phone.

## Agent-to-agent messaging

Agents use the same `ttal send` shape for other agents:

```
cat <<'EOF' | ttal send --to {agent}
Can you research the auth library options?
EOF

cat <<'EOF' | ttal send --to {agent}
Task complete
EOF
```

When sent from an agent session (where `TTAL_AGENT_NAME` is set), the recipient sees attribution:

```text
<- {agent} [14:32:05] The design plan is ready for review.

<i>--- Reply with:
cat <<'EOF' | ttal send --to {agent}
your message
EOF</i>
```

## CC control commands

From Telegram, you can send bot commands to control the agent's Claude Code session:

- `/new` — start a new conversation
- `/compact` — compact the context
- `/wait` — pause the agent
