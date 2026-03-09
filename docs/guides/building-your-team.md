---
title: Building Your Team
description: Team composition patterns for different needs
---

ttal supports different team shapes depending on your workflow. Start simple and add agents as your needs grow.

## Team patterns

### Solo developer — 1 orchestrator + N workers

The simplest setup. One persistent agent handles orchestration, and workers are spawned for individual tasks.

```bash
ttal agent add kestrel --role orchestrator
```

Kestrel manages your tasks, responds to Telegram messages, and spawns workers when you run `ttal task execute`. Each worker is isolated in its own tmux session and git worktree.

**Best for:** Individual developers who want mobile access and task-driven workflows.

### Research team — researcher + designer + workers

Add specialized agents for research and design phases before execution.

```bash
ttal agent add athena --role researcher
ttal agent add inke --role designer
ttal agent add kestrel --role orchestrator
```

Set roles in agent frontmatter so routing resolves automatically:

```bash
ttal agent modify inke role:designer
ttal agent modify athena role:researcher
```

Now `ttal task route <uuid> --to inke` goes to Inke (role: designer), `ttal task route <uuid> --to athena` goes to Athena (role: researcher), and `ttal task execute <uuid>` spawns a worker.

**Best for:** Complex projects where tasks benefit from investigation and planning before implementation.

### Full lifecycle — orchestrator + researcher + designer + reviewer + workers

The full setup with dedicated review agents.

```bash
ttal agent add kestrel --role orchestrator
ttal agent add athena --role researcher
ttal agent add inke --role designer
ttal agent add sage --role reviewer
```

Workers implement features, create PRs, and specialized review agents analyze the code. The reviewer posts verdicts, the worker triages, and you merge from Telegram.

**Best for:** Teams that want autonomous, auditable delivery pipelines.

## Setting up agents

### 1. Create a Telegram bot for each agent

Talk to [@BotFather](https://t.me/BotFather) on Telegram:
1. Send `/newbot`
2. Choose a name (e.g., "Athena Research Agent")
3. Choose a username (e.g., `athena_research_bot`)
4. Copy the bot token

### 2. Register the agent

```bash
ttal agent add athena --role researcher
```

### 3. Add the bot token to config

```toml
[teams.default.agents.athena]
bot_token = "123456:ABC..."
```

### 4. Start a chat

Open a DM with your new bot on Telegram. Send it a message — the daemon will deliver it to the agent's tmux session.

## Multi-team setup

For completely separate contexts (different projects, different taskwarrior instances):

```toml
[teams.personal]
taskrc = "~/.taskrc"

[teams.work]
taskrc = "~/.task-work/taskrc"
```

Each team has its own:
- Taskwarrior database
- Agent roster
- Bot tokens
- Data directory

Switch teams with `TTAL_TEAM=work ttal today list`.

## When to split into multiple teams

**Keep one team when:**
- All your projects share the same agents
- You want a unified task list
- One Telegram chat per agent is enough

**Split into teams when:**
- You have distinct contexts (personal vs work)
- Different projects need different runtimes
- You want separate taskwarrior databases
