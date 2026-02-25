---
title: About
description: Neil builds infrastructure for knowledge workers to delegate to AI
---

I'm Neil, co-founder at [GuionAI](https://guion.ai). I handle the backend, infrastructure, and DevOps across all our products—Kubernetes, serverless, databases, the plumbing that makes things work.

This lab is where I share what I'm building.

## The Problem I'm Solving

Knowledge work is broken. We're drowning in context-switching, tab-hopping, and manual coordination. Every tool promises to help, but most just add another surface to check.

I wanted something different: capture everything, think clearly, delegate to agents. Not as separate apps, but as a connected system.

## The GuionAI Ecosystem

### FlickNote

[FlickNote](https://flicknote.app/) is a modern inbox for the AI era.

- **Capture from everywhere** — voice memos, links, messages, meeting notes
- **AI auto-tagging** — it organizes so you don't have to
- **MCP integration** — agents can read your knowledge base, meetings, and links

Think of it as the input layer. Everything important goes here.

### TTAL /tiːtæl/ (The Taskwarrior Agents Lab)

This site. An open lab exploring agent-driven task orchestration.

- **The stack**: Taskwarrior + Zellij + Claude Code
- **Daily guides** on patterns that actually work
- **Upcoming iOS app** with full Taskwarrior support, Taskchampion sync, and OpenClaw agents

This is the execution layer. Agents read tasks, do work, report back.

## The Connection

Here's what makes this interesting: **FlickNote and TTAL are designed to work together**.

```
FlickNote captures insights
       ↓
TTAL agents read & act (via MCP)
       ↓
Back to your tasks & annotations
```

- Agents can read your FlickNote knowledge base and meeting records
- The TTAL iOS app (coming soon) links FlickNote items to task annotations
- Your captured knowledge becomes agent context

The goal: you capture once, agents use it forever. No re-explaining. No copying context between tools.

## Why I'm Sharing This

I've spent months building and refining this system. Most of it happens in private repos and internal tools. But the patterns are useful beyond my own work.

This lab is where I document what works, what doesn't, and how the pieces fit together. If you're building similar systems—or just curious about agent orchestration—maybe something here saves you time.

## Get Started

- **[Day 1: The Dream Setup](/guides/day-1-the-dream-setup/)** — why Taskwarrior is the secret sauce
- **[AIOps Project](/projects/aiops/)** — the real working system
- **[FlickNote](https://flicknote.app/)** — the capture layer

---

Questions or ideas? Find me on [Telegram](https://t.me/neilbbN) or [email](mailto:bn0010100@gmail.com).
