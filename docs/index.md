---
layout: home

hero:
  name: TTAL
  text: Build your own software company, managed by AI agents.
  tagline: The coordination layer for Claude Code, OpenCode, and Codex CLI.
  actions:
    - theme: brand
      text: Get Started
      link: /docs/getting-started
    - theme: alt
      text: View on Codeberg
      link: https://codeberg.org/clawteam/ttal-cli

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
import Typer from './.vitepress/components/Typer.vue'
import PricingCards from './.vitepress/components/PricingCards.vue'
</script>

<div class="typer-line">
  <Typer :words="['Telegram', 'Teamwork', 'Taskwarrior']" />
</div>

<p class="vision-sub">
Whether you're a developer, a student, or anyone with a problem to solve — TTAL gives you a team of AI agents that research, plan, build, review, and ship.
</p>

---

## Architecture

```
┌─────────────────────────────────────────────┐
│  Human (Telegram — from anywhere)           │
├─────────────────────────────────────────────┤
│  ttal (coordination, messaging, routing)    │
├─────────────────────────────────────────────┤
│  Coding harness (optional — oh-my-opencode) │
├─────────────────────────────────────────────┤
│  Runtime (Claude Code / OpenCode / Codex)   │
└─────────────────────────────────────────────┘
```

**One bot per agent.** Each agent is its own Telegram bot, its own DM chat. Talk to your researcher about research while your designer designs.

---

## How TTAL compares

TTAL doesn't replace Claude Code — it makes it a team player.

| Capability | TTAL | claude-flow / claude-squad | Praktor | Claudegram |
|---|:---:|:---:|:---:|:---:|
| Multi-agent coordination | ✓ | ✓ | ✓ | - |
| Bidirectional Telegram | ✓ | - | ✓ | ✓ |
| Multimodal input | ✓ | - | - | ✓ |
| TTS / STT | ✓ | - | - | ✓ |
| Task management | ✓ | - | - | - |
| Interactive questions | ✓ | - | - | ✓ |
| Autonomous PR workflow | ✓ | - | - | - |

Competitors build chat assistants. TTAL builds autonomous team members who own the full delivery pipeline.

---

## Get started

```bash
# Install
git clone https://codeberg.org/clawteam/ttal-cli.git
cd ttal-cli && make install

# Set up hooks and daemon
ttal worker install
ttal daemon install

# Add your first agent
ttal agent add kestrel +core +backend
```

**[Read the docs →](/docs/getting-started)** | **[View source on Codeberg →](https://codeberg.org/clawteam/ttal-cli)**

---

## Plans

<PricingCards />
