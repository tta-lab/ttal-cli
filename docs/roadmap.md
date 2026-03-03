---
title: Roadmap
description: What's coming next for ttal
---

# Roadmap

ttal is actively developed. Here's what's on the horizon.

## v1.x — Current

The foundation: multi-agent orchestration over Telegram with Claude Code, OpenCode, and Codex CLI support.

- **Agent lifecycle** — spawn, monitor, and clean up coding workers
- **Telegram bridge** — bidirectional messaging between humans and agents
- **PR workflow** — create, review, merge PRs from your phone
- **Task routing** — design, research, test, execute pipeline
- **Voice** — text-to-speech via Kokoro for agent responses
- **Multi-runtime** — Claude Code, OpenCode, Codex CLI
- **Starter templates** — `ttal init` scaffolds for new teams

### v1.x target: multi-runtime GA

The v1.x goal is making **OpenCode and Codex CLI support production-ready** alongside Claude Code — so all three runtimes are fully supported for spawning, messaging, PR workflows, and review.

**In progress:**
- OpenCode and Codex CLI runtime parity (spawn, review, yolo modes)
- Filesystem-based agent discovery (replacing DB-stored agents)
- `ttal init` interactive scaffold picker
- Configurable prompts for all task routing

## v2.0 — Matrix Protocol

Replace Telegram with [Matrix](https://matrix.org) as the primary communication layer.

**Why Matrix:**
- **Unlimited bots** — Telegram limits bot interactions; Matrix has no such constraints
- **Rich markdown** — full markdown rendering in messages (code blocks, tables, formatting)
- **Self-hosted** — run your own Synapse server, full data sovereignty
- **E2EE** — end-to-end encryption for agent communications
- **Rooms as channels** — dedicated rooms per agent, per project, per workflow
- **Bridges** — Matrix bridges to Telegram, Slack, Discord for gradual migration

**Planned changes:**
- Matrix adapter alongside existing Telegram adapter
- Room-per-agent architecture
- Threaded conversations for task context
- File sharing (plans, research docs, screenshots) directly in chat

## v2.x — Sandboxed Execution

Containerize worker execution for security and reproducibility.

**Why sandboxing:**
- **Isolation** — workers can't affect host system or other workers
- **Reproducibility** — consistent environments across runs
- **Security** — untrusted code execution in a safe boundary
- **Resource limits** — cap CPU, memory, disk per worker

**Planned approach:**
- Container-per-worker (lightweight, ephemeral)
- Pre-built images with common toolchains (Node, Go, Rust, Python)
- Volume mounts for git worktrees
- Network policy controls

