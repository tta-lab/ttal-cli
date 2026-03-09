---
layout: home

hero:
  text: One binary to run your autonomous software company.
  tagline: Built on the coding agents you love. Managed from your phone. The full cycle runs itself.
  actions:
    - theme: brand
      text: Get Started
      link: /docs/getting-started
    - theme: alt
      text: View on GitHub
      link: https://github.com/tta-lab/ttal-cli

features:
  - icon: "📱"
    title: Mobile Command Center
    details: Each agent is its own Telegram bot, its own DM chat. Send text, voice, photos, files. Approve PRs from your phone. Interactive question buttons when agents need input.
  - icon: "🐱"
    title: Persistent Agent Team
    details: Named agents with roles, voices, and memory. Your orchestrator routes tasks. Your researcher investigates. Your designer plans. They talk to each other.
  - icon: "📋"
    title: Task-Driven Pipeline
    details: Research → Design → Execute → Review → Merge. Each phase produces artifacts. Taskwarrior integration with enrichment hooks. Quality from structure, not babysitting.
  - icon: "⚡"
    title: Multi-Runtime Flexibility
    details: Claude Code, OpenCode, Codex CLI — mix runtimes across your team. Workers spawn in isolated git worktrees. 6 specialized reviewers on every PR.
---

<script setup>
import TerminalDemo from './.vitepress/components/TerminalDemo.vue'
import AgentRoster from './.vitepress/components/AgentRoster.vue'
import HowItWorks from './.vitepress/components/HowItWorks.vue'
import InstallTabs from './.vitepress/components/InstallTabs.vue'
import PricingCards from './.vitepress/components/PricingCards.vue'
import FaqSection from './.vitepress/components/FaqSection.vue'
</script>

## See it in action

<TerminalDemo />

---

## Meet Your Team

Your agents aren't anonymous processes — they're persistent team members with names, personalities, voices, and Telegram chats.

<AgentRoster />

---

## How it works

<HowItWorks />

---

## Two-Plane Architecture

TTAL runs your team on two planes:

**Manager Plane** — Long-running agents with specialized roles. Your researcher, designer, orchestrator. They persist across sessions, have memory, and coordinate via agent-to-agent messaging.

**Worker Plane** — Short-lived coders and reviewers. Spawned on demand, one per task, isolated in git worktrees. Multiple workers run in parallel. They implement, review, triage, and merge — then they're done.

```text
┌─────────────────────────────────────────────────┐
│  You (Telegram — from anywhere)                 │
├────────────────────┬────────────────────────────┤
│  Manager Plane     │  Worker Plane              │
│  ───────────────   │  ─────────────             │
│  🦉 Researcher     │  👷 Coder (task A)         │
│  🐙 Designer       │  👷 Coder (task B)         │
│  🐱 Orchestrator   │  🔍 Reviewer (PR #42)     │
│  🦅 Lifecycle      │  👷 Coder (task C)         │
│  (long-running)    │  (short-lived, parallel)   │
├────────────────────┴────────────────────────────┤
│  TTAL (coordination, messaging, task routing)   │
├─────────────────────────────────────────────────┤
│  Runtime (Claude Code / OpenCode / Codex)       │
└─────────────────────────────────────────────────┘
```

**One bot per agent.** Each manager agent is its own Telegram bot, its own DM chat. Talk to your researcher about research while your designer designs.

> **Why these choices?** Read the [Philosophy](/blog/philosophy) — the design decisions behind ttal and why they exist.

---

## How TTAL compares

TTAL doesn't replace your coding agent — it makes it a team player.

| Capability | TTAL | Paperclip | claude-flow / claude-squad | OpenClaw | Claudegram |
|---|:---:|:---:|:---:|:---:|:---:|
| Multi-agent coordination | ✓ | ✓ | ✓ | ✓ | - |
| Multi-runtime (CC + OpenCode + Codex) | ✓ | ✓ | - | - | - |
| OpenClaw as manager runtime | ✓ | - | - | n/a | - |
| Zero infrastructure (no database) | ✓ | - | ✓ | - | - |
| Bidirectional Telegram | ✓ | - | - | - | ✓ |
| Multimodal input | ✓ | - | - | - | ✓ |
| TTS / STT | ✓ | - | - | - | ✓ |
| Task management | ✓ | ✓ | - | - | - |
| Interactive questions | ✓ | - | - | - | ✓ |
| Autonomous PR workflow | ✓ | - | - | - | - |
| Git worktree isolation | ✓ | - | - | - | - |
| Multi-team + cross-team comms | ✓ | - | - | - | - |

The closest competitor is **Paperclip** (12K stars) — a company-as-abstraction platform with a React dashboard, goal hierarchy, and budget controls. It supports multiple runtimes including Claude Code, OpenCode, and Codex. Where it diverges: Paperclip requires PostgreSQL and a Node.js server; TTAL is a single Go binary with no database. Paperclip models your team as an org chart; TTAL models it as two planes — managers and workers — coordinated through git-native workflows and direct Telegram access.

TTAL is the only orchestrator that combines CLI-first zero-infra deployment, deep git integration (worktrees, PR lifecycle, PR watcher), and cross-team communication within a single org — something no other orchestrator supports.

Competitors build chat assistants or company simulators. TTAL builds autonomous software teams who own the full delivery pipeline.

---

## Install

<InstallTabs />

---

## Questions & Answers

<FaqSection />

---

<PricingCards />
