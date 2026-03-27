---
layout: home

hero:
  text: "Hi, I'm TTal — yes, a snail."
  tagline: Slow to break things, fast to ship them. One binary. No cloud. Drop me in your terminal.
  actions:
    - theme: brand
      text: Get Started
      link: /docs/getting-started
    - theme: alt
      text: View on GitHub
      link: https://github.com/tta-lab/ttal-cli

features:
  - icon: "📱"
    title: Talk to me from anywhere
    details: Each agent is its own Telegram bot. Send tasks from your couch.
  - icon: "🐱"
    title: I remember my team
    details: Named agents with roles and memory. They talk to each other so you don't have to.
  - icon: "📋"
    title: The pipeline runs itself
    details: Research → Design → Execute → Review → Merge. You approve from your phone.
  - icon: "⚡"
    title: Swap any brain in
    details: Claude Code, Codex, whatever's next. The pipeline doesn't care which LLM thinks.
---

<script setup>
import TerminalDemo from './.vitepress/components/TerminalDemo.vue'
import AgentRoster from './.vitepress/components/AgentRoster.vue'
import HowItWorks from './.vitepress/components/HowItWorks.vue'
import InstallTabs from './.vitepress/components/InstallTabs.vue'
import PricingCards from './.vitepress/components/PricingCards.vue'
import FaqSection from './.vitepress/components/FaqSection.vue'
import StatsBar from './.vitepress/components/StatsBar.vue'
</script>

<StatsBar />

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

**[TTal](https://github.com/tta-lab/ttal-cli) coordinates. [logos](https://github.com/tta-lab/logos) thinks. CC native sandbox isolates.**

Two planes. Everything else follows from that.

**Manager Plane** — Long-running agents with specialized roles. Researcher, designer, orchestrator. They persist across sessions, have memory, and coordinate via agent-to-agent messaging.

**Worker Plane** — Short-lived coders and reviewers. One per task, isolated in git worktrees. Multiple workers run in parallel. They implement, review, triage, and merge — then they're done.

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
│  Runtime (Claude Code / Codex)                  │
└─────────────────────────────────────────────────┘
```

**One bot per agent.** Each manager is its own Telegram bot, its own DM. Talk to your researcher while your workers ship.

> **Why these choices?** Read the [Philosophy](/blog/philosophy) — the design decisions behind ttal and why they exist.

---

## How TTAL compares

TTAL doesn't replace your coding agent — it makes it a team player.

| Capability | TTAL | Paperclip |
|---|:---:|:---:|
| Multi-agent coordination | ✓ | ✓ |
| Zero infrastructure (no database) | ✓ | - |
| Bidirectional Telegram | ✓ | - |
| Autonomous PR workflow | ✓ | - |

The closest competitor is **Paperclip** (12K stars) — multi-runtime support, goal hierarchy, React dashboard. It requires PostgreSQL and a Node.js server. TTAL is a single Go binary with no database. Paperclip models your team as an org chart; TTAL models it as two planes — coordinated through git-native workflows and direct Telegram access.

Competitors build chat assistants or company simulators. TTAL builds autonomous software teams who own the full delivery pipeline.

---

## Install

<InstallTabs />

---

## Questions & Answers

<FaqSection />

---

<PricingCards />
