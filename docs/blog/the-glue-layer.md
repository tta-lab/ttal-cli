---
title: "The Glue Layer"
description: How ttal evolved from OpenClaw scripts to a local-first agent orchestration CLI
---

[The Dream Setup](/blog/the-dream-setup) ended with a promise: Taskwarrior hooks fire events, and those events can trigger agent workflows.

But here's what I glossed over: events alone are just noise.

A hook fires. Then what? Where does the event go? Who decides what to do with it? What if the action requires multiple steps, or human approval, or coordination across systems?

You need a glue layer. Something that sits between "event happened" and "work got done."

## The Missing Middle

Picture the stack:

```
┌─────────────────────────────────────┐
│  Human Intent (Taskwarrior tasks)   │  ← The Dream Setup
├─────────────────────────────────────┤
│  ??? (something goes here)          │  ← This post
├─────────────────────────────────────┤
│  Agent Execution (tmux sessions)    │
└─────────────────────────────────────┘
```

The Dream Setup covered the top layer: tasks as structured data, hooks as events. This post covers the middle: the orchestration layer that makes autonomous workflows possible. Without it, you're manually wiring every hook to every action.

## Where It Started: OpenClaw

The first version of this stack used [OpenClaw](https://github.com/openclaw/openclaw) as the glue layer. OpenClaw is a multi-channel AI assistant platform — it handles webhooks, sessions, workflow orchestration, and messaging channel adapters (Telegram, Slack, WhatsApp).

The setup worked:

```
task start → on-modify hook → curl webhook → OpenClaw → spawn agent in Zellij
```

OpenClaw received the Taskwarrior event, decided what to do, and spawned coding agents in Zellij sessions. Manager agents ran inside OpenClaw, execution agents ran in Zellij.

Having a long-running agent team was genuinely useful. The manager agents accumulated context about the project. The execution agents stayed focused on individual tasks. The separation of stateful managers and stateless workers was the right architecture.

## The Insight: Untie the Knot

After running this setup for a while, something became clear: **the orchestration layer doesn't need to be OpenClaw**.

The core requirements are simple:
- Listen for Taskwarrior hook events
- Enrich tasks with project context (path, branch)
- Spawn a coding agent in an isolated workspace
- Route messages between agents and humans via Telegram
- Clean up when work is done

That's not a platform. That's a daemon with a few socket handlers and some tmux commands.

More importantly: tying the orchestration layer to any specific platform meant the whole stack failed when that platform had issues. The simpler the glue, the more reliable the system.

## What ttal Became

ttal (originally "Task-to-Agent Layer") started as a collection of shell scripts around OpenClaw. Over time, those scripts got replaced with a Go CLI that handles the full lifecycle directly.

The key components:

| Component | What it does |
|-----------|--------------|
| **Daemon** | Long-running process; handles socket connections, Telegram, fsnotify watchers |
| **on-add-ttal hook** | Fires when a task is created; enriches with project path and branch |
| **Worker spawn** | `ttal task execute <uuid>` → tmux session + git worktree + coding agent |
| **Cleanup watcher** | Watches for worker completion; closes session, removes worktree, marks task done |
| **ttal send** | Agent-to-agent messaging via daemon socket |

The architecture is the same manager/worker split that worked in OpenClaw — but now it runs entirely locally, without any cloud dependency.

## Two Types of Agents (Still)

The manager/worker distinction survived the migration. It's too useful to abandon.

**Manager plane** — Long-running agents (orchestrator, researcher, designer). They run in Claude Code sessions that persist across reboots. They have memory, accumulated project context, and persistent identities. They coordinate via Telegram and agent-to-agent messaging.

**Worker plane** — Short-lived coders and reviewers. Spawned on demand per task, isolated in git worktrees within tmux sessions. They run in parallel, implement → review → merge → done, then they're gone.

```
┌─────────────────────────────────────────────────────────┐
│                   Manager Plane                         │
│  ┌─────────────┐  ┌─────────────┐  ┌─────────────────┐  │
│  │ Orchestrator│  │ Researcher  │  │ Designer        │  │
│  └─────────────┘  └─────────────┘  └────────┬────────┘  │
│                                             │           │
└─────────────────────────────────────────────┼───────────┘
                                              │ ttal task execute
                                              ▼
┌─────────────────────────────────────────────────────────┐
│              Worker Plane (tmux + worktree)              │
│  ┌──────────┐  ┌────────┐  ┌──────────┐                  │
│  │ Coder    │→ │Reviewer│→ │ Merge    │ → done           │
│  └──────────┘  └────────┘  └──────────┘                  │
└─────────────────────────────────────────────────────────┘
```

Managers are **stateful** — they learn, remember, improve over time. Workers are **stateless** — disposable experts with perfect context injected at spawn time.

## Memory: Who Remembers What

**Manager agents** maintain:
- **Identity** — Personality, communication style, preferences (in `CLAUDE.md` frontmatter)
- **Long-term memory** — Past decisions, learned patterns (in `memory/` directories)
- **Workflow knowledge** — How things are done here, what worked before

**Worker agents** receive all context they need at spawn time:

```
Here's the task: Fix auth timeout bug
Here's the context: Token refresh not triggering (from annotations)
Here's how we do things: Always add tests, use guard clauses (from CLAUDE.md)
```

The worker doesn't need to remember your last three projects. It needs exactly the right context for *this* task. Managers curate that context and inject it at spawn time.

## Any Runtime Works

One of ttal's design goals: the orchestration layer should be runtime-agnostic.

Workers run in tmux sessions. What runs *inside* those sessions is configurable — Claude Code, Codex CLI, or anything else. The spawn logic injects a task prompt and sets up the workspace. The coding agent is just a process that reads the prompt and writes code.

This means the stack doesn't break when a new coding agent appears. Swap the runtime; keep the orchestration.

## The Full Loop

```
You: ttal task execute a1b2c3d4
  ↓
ttal daemon: reads task, resolves project path
  ↓
worker spawn: new git worktree + tmux session
  ↓
coding agent: reads task prompt, implements, creates PR
  ↓
reviewer: posts verdict as PR comment
  ↓
ttal pr merge: drops cleanup request to ~/.ttal/cleanup/
  ↓
daemon cleanup watcher: closes session, removes worktree, marks task done
  ↓
You: notification on Telegram (PR merged, task done)
```

You delegated a task from the terminal. You got a notification on your phone. You never watched a terminal.

## What Didn't Change

The architecture that worked in OpenClaw is still the architecture in ttal:

- Tasks as structured data (Taskwarrior)
- Annotations as the shared communication channel
- Manager/worker split with different memory models
- Humans in the loop via approval gates (PR review, not automation)

What changed: the glue layer got simpler. No cloud platform. No webhook infrastructure. A daemon, a socket, and some tmux commands.

The simpler the glue, the more reliable the system. That's the lesson OpenClaw taught.
