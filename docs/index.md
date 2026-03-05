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
  - icon: "\U0001F4F1"
    title: Telegram Bridge
    details: Mobile-first agent management. Each agent is its own bot, its own DM chat. Send messages, check status, approve PRs — from your phone.
  - icon: "\U0001F4CE"
    title: Multimodal Input
    details: Send text, voice, photos, screenshots, files, links. The bot transcribes voice, downloads images, and delivers everything in context.
  - icon: "\u2B50"
    title: Persistent Agent Identity
    details: Names, roles, voices, routing. Your agents aren't anonymous processes — they're persistent team members with memory.
  - icon: "\u2699\uFE0F"
    title: Multi-Runtime Support
    details: Claude Code (stable), OpenCode and Codex CLI (experimental — shipping soon). TTAL doesn't care what's in the tmux session.
  - icon: "\U0001F4CB"
    title: Taskwarrior Integration
    details: Task-driven workflows. Enrichment hooks auto-populate metadata. Research → design → execute pipeline.
  - icon: "\U0001F399\uFE0F"
    title: Voice I/O
    details: Local TTS/STT via Kokoro + Whisper on Apple Silicon. Per-agent voices. No cloud API keys.
  - icon: "\U0001F4AC"
    title: Interactive Questions
    details: Agent needs input? It sends the question to Telegram with inline buttons. Tap a choice or type a custom answer.
  - icon: "\U0001F500"
    title: Agent-to-Agent Messaging
    details: Horizontal peer-to-peer communication. Agents consult each other, delegate work, and share context.
  - icon: "\u2705"
    title: Autonomous PR Workflow
    details: Implement → PR → 6 specialized reviewers → triage → merge. Full delivery pipeline, fully auditable.
---

<script setup>
import PricingCards from './.vitepress/components/PricingCards.vue'
import FaqSection from './.vitepress/components/FaqSection.vue'
</script>

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

---

## How TTAL compares

TTAL doesn't replace your coding agent — it makes it a team player.

| Capability | TTAL | claude-flow / claude-squad | OpenClaw | Claudegram |
|---|:---:|:---:|:---:|:---:|
| Multi-agent coordination | ✓ | ✓ | ✓ | - |
| Multi-runtime (Claude Code + OpenCode + Codex) | ✓ | - | - | - |
| OpenClaw as manager runtime | ✓ | - | n/a | - |
| Bidirectional Telegram | ✓ | - | - | ✓ |
| Multimodal input | ✓ | - | - | ✓ |
| TTS / STT | ✓ | - | - | ✓ |
| Task management | ✓ | - | - | - |
| Interactive questions | ✓ | - | - | ✓ |
| Autonomous PR workflow | ✓ | - | - | - |

TTAL is the only orchestrator that supports **three agent runtimes** — Claude Code (stable), OpenCode, and Codex CLI — and even runs **OpenClaw as the manager plane runtime**, giving you the flexibility to mix runtimes across your team.

Competitors build chat assistants. TTAL builds autonomous team members who own the full delivery pipeline.

---

## Questions & Answers

<FaqSection />

---

## Get started

```bash
# Install
git clone https://github.com/tta-lab/ttal-cli.git
cd ttal-cli && make install

# Set up hooks and daemon
ttal worker install
ttal daemon install

# Add your first agent
ttal agent add kestrel +core +backend
```

**[Read the docs →](/docs/getting-started)** | **[View source on GitHub →](https://github.com/tta-lab/ttal-cli)**

---

<PricingCards />
