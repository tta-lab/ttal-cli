---
title: Agents
description: Agent management and identity in ttal
---

Agents in ttal are persistent identities — not anonymous processes. Each agent has a name, tags, and optionally an emoji, creature type, and voice.

## Adding agents

```bash
ttal agent add kestrel +core +backend
```

This creates an agent named `kestrel` with the tags `core` and `backend`.

## Agent identity

Agents can have rich identity attributes:

```bash
ttal agent add athena +research +design
ttal agent modify athena -- path:/home/neil/clawd/.athena
```

The agent's `path` is the working directory for its tmux session.

## Listing agents

```bash
# List all agents
ttal agent list

# Filter by tag
ttal agent list +research
```

## Agent info

```bash
ttal agent info kestrel
```

Shows the agent's tags, status, path, and matching projects. Projects match when they share at least one tag with the agent.

## Modifying agents

Use `--` before modifications to prevent `-tag` from being interpreted as a flag:

```bash
# Add and remove tags
ttal agent modify kestrel -- +infrastructure -legacy

# Change path
ttal agent modify kestrel -- path:/new/workspace/path

# Combine operations
ttal agent modify kestrel -- path:/new/path +research -demo
```

## Agent status

Agents have three status values:

- `idle` — available for task assignment
- `busy` — currently working
- `paused` — temporarily disabled

```bash
ttal agent status kestrel busy
```

## Tag-based routing

Tags drive how tasks get routed to agents. When a task is created with tags, ttal matches them against registered agents:

```bash
# Agent with research tag
ttal agent add athena +research

# Task tagged +research routes to athena
task add "Investigate auth options" +research
```

Tags are case-insensitive and stored as lowercase.

## One bot per agent

Each agent gets its own Telegram bot and its own DM chat. This means you can talk to your researcher about research while your designer designs — like messaging actual team members, not @-routing in a single chat.

Configure bot tokens in `config.toml`:

```toml
[teams.default.agents.kestrel]
bot_token = "123456:ABC..."
```

Create a bot via [@BotFather](https://t.me/BotFather) on Telegram for each agent.
