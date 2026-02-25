---
title: AIOps System
description: Taskwarrior + Zellij + Claude Code, orchestrated with hooks and agents
---

This is the real working system behind the guide series—and it's not just for programmers.

## The Idea

What if you could type `task start` and have everything set itself up? Workspace opens. Context loads. AI agent starts working. When you're done, it all cleans up.

Like GitHub Codespaces, but local. Under your control. And extensible to any workflow—coding, content creation, sales pipelines, personal goals.

![Taskwarrior TUI showing agent-managed tasks](/screenshots/taskwarrior-tui.png)
*Every task here was created and managed by agents.*

## The Stack

- **[Taskwarrior](https://taskwarrior.org/)** — Your tasks as structured data. Hooks that fire when things change.
- **[OpenClaw](https://github.com/openclaw/openclaw)** — Talk to your agents from WhatsApp, Telegram, Slack. Webhooks and workflows.
- **[Zellij](https://zellij.dev/)** — Terminal sessions that spawn, manage context, and clean up automatically.
- **[Claude Code](https://claude.com/product/claude-code)** — AI that actually writes code. (Or swap for opencode, aider, crush—your choice.)

Desktop apps (browser automation, Excel, PowerPoint) plug into the same webhook system alongside terminal agents.

## How It Works

The short version:

```
task start → hook fires → workspace spawns → agent works → task done → cleanup
```

The key insight: **annotations become a shared communication channel**. Humans, orchestrator agents, and workers all write to the same place. Query any task, see the full conversation. No context scattered across Slack, email, and docs.

## Current Status

The system is in active use. I'm documenting it piece by piece in the [guide series](/guides/day-1-the-dream-setup/).

## Components

| Component | Role | Status |
|-----------|------|--------|
| [Taskwarrior](https://taskwarrior.org/) | Structured task data + hooks | Stable |
| [Zellij](https://zellij.dev/) | Session/workspace management | Stable |
| [Claude Code](https://claude.com/product/claude-code) | AI development agent | Stable |
| [OpenClaw](https://github.com/openclaw/openclaw) | Multi-channel messaging + orchestration | Stable |

## Follow Along

The guide series covers:
1. [Day 1: The Dream Setup](/guides/day-1-the-dream-setup/) - Why Taskwarrior is the secret sauce
2. [Day 2: OpenClaw Overview](/guides/day-2-openclaw-overview/) - Webhooks, workflows, orchestration
3. [Day 3: Zellij + Coding Agents](/guides/day-3-zellij-coding-agents/) - Session management, agent coordination
4. [Day 4: Taskwarrior Deep Dive](/guides/day-4-taskwarrior-deep-dive/) - UDAs, hooks, queries, advanced patterns
5. [Day 5+: Topics TBD](/guides/day-5-tbd/) - Driven by reader interest
