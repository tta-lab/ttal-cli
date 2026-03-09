# AIOps System

> Taskwarrior + terminal sessions + Claude Code, orchestrated with hooks and agents

## The Idea

What if you could type `task start` and have everything set itself up? Workspace opens. Context loads. AI agent starts working. When you're done, it all cleans up.

Like GitHub Codespaces, but local. Under your control. And extensible to any workflow—coding, content creation, sales pipelines, personal goals.

## The Stack

- **[Taskwarrior](https://taskwarrior.org/)** — Your tasks as structured data. Hooks that fire when things change.
- **Telegram** — Talk to your agents from your phone. Messages delivered to agent terminal sessions.
- **tmux** — Terminal sessions that spawn, manage context, and clean up automatically.
- **[Claude Code](https://claude.com/product/claude-code)** — AI that actually writes code. (Or swap for opencode, aider, crush—your choice.)

Desktop apps (browser automation, Excel, PowerPoint) plug into the same webhook system alongside terminal agents.

## How It Works

The short version:

```
task start → hook fires → workspace spawns → agent works → task done → cleanup
```

The key insight: **annotations become a shared communication channel**. Humans, orchestrator agents, and workers all write to the same place. Query any task, see the full conversation. No context scattered across Slack, email, and docs.

## Further Reading

- [Building Your Team](guides/building-your-team.md) — Setting up agents and roles
- [PR Review Workflow](guides/pr-review-workflow.md) — Automated review with specialized agents
