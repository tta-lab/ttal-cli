---
title: About
description: The story behind ttal — manage your coding agents from your phone
---

I'm Neil, co-founder at [GuionAI](https://guion.ai). I handle the backend, infrastructure, and DevOps across all our products — Kubernetes, serverless, databases, the plumbing that makes things work.

## The Problem I'm Solving

Knowledge work is broken. We're drowning in context-switching, tab-hopping, and manual coordination. Coding agents are powerful but terminal-bound — you have to be at your desk to use them.

I wanted something different: manage a team of coding agents from anywhere, with task-driven workflows, voice communication, and mobile access. Not another chat assistant, but autonomous team members who own the full delivery pipeline.

## ttal /tiːtæl/

**Manage your coding agents from your phone.**

ttal is the coordination layer for Claude Code, OpenCode, and Codex CLI. It adds multi-agent orchestration, Telegram messaging, Taskwarrior integration, and voice I/O on top of whatever coding runtime you use.

- **Open source** — MIT license, [hosted on Codeberg](https://codeberg.org/clawteam/ttal-cli)
- **Mobile-first** — each agent is its own Telegram bot, manage everything from your phone
- **Task-driven** — research → design → execute pipeline with automatic context flow
- **Local voice** — TTS/STT via Kokoro + Whisper on Apple Silicon, no cloud API keys
- **Runtime-agnostic** — Claude Code today, any terminal-based coding CLI tomorrow

## The GuionAI Ecosystem

### FlickNote

[FlickNote](https://flicknote.app/) is a modern inbox for the AI era.

- **Capture from everywhere** — voice memos, links, messages, meeting notes
- **AI auto-tagging** — it organizes so you don't have to
- **MCP integration** — agents can read your knowledge base, meetings, and links

Think of it as the input layer. Everything important goes here.

### The Connection

FlickNote and ttal are designed to work together:

```
FlickNote captures insights
       ↓
ttal agents read & act (via MCP)
       ↓
Back to your tasks & annotations
```

You capture once, agents use it forever. No re-explaining. No copying context between tools.

## Get Started

- **[Getting Started](/docs/getting-started/)** — install ttal and set up your first agent
- **[Documentation](/docs/configuration/)** — configuration, commands, and workflows
- **[Blog](/blog/day-1-the-dream-setup/)** — the journey from manual to autonomous
- **[FlickNote](https://flicknote.app/)** — the capture layer

---

Questions or ideas? Find me on [Telegram](https://t.me/neilbbN) or [email](mailto:bn0010100@gmail.com).
