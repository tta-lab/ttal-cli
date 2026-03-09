---
title: Philosophy
description: The design decisions behind ttal and why they exist
---

# Philosophy

ttal is opinionated. Here's why.

---

## 1. Remove the Human from the Loop — But Keep Quality

The problem with most AI coding setups isn't that agents are bad. It's that you spawn a coding agent, then watch it work for 20 minutes. That's not leverage — that's a fancier terminal.

ttal's answer is a structured pipeline: research → design → execute → review → merge. Each phase produces artifacts. The researcher writes findings. The designer writes a plan. The coder implements against that plan. Six specialized reviewers post comments on every PR. Quality comes from the pipeline, not from supervision.

The review step is advisory by design. Reviewers post verdicts (`LGTM` or `NEEDS_WORK`) with specific comments. The coder triages the remaining issues and merges. No human needs to read every line — they just need to check the result.

The human's job shrinks to three things: decide what to build, approve the plan, check the outcome. Not: watch the agent type.

---

## 2. CLI-First, Local-First

Everything in ttal runs on local Unix tools. Not cloud services, not databases, not SaaS APIs.

**Taskwarrior** for tasks — not Jira, not Linear, not a hosted database. It's a local binary with JSON export, hooks, and 30 years of Unix philosophy behind it. `task export` gives you structured data in one command.

**TOML files** for config — `projects.toml`, `config.toml`. Human-readable, git-friendly, editable with any text editor. No migration scripts.

**tmux** for isolation — each worker gets a tmux session and a git worktree. Shell-native, zero infrastructure overhead. Workers run in parallel without touching each other.

**FlickNote** for knowledge — notes stored locally, queried via CLI. No account required.

The principle: if your laptop works, ttal works. No cloud dependency for core operations. Telegram is the only network dependency, and it's for convenience — mobile access from anywhere — not correctness. The system works fine without it.

Contrast with alternatives: cloud-based agent platforms require accounts, API keys, internet connectivity. ttal requires `brew install ttal`.

---

## 3. A System That Accelerates Itself

ttal's most distinctive property: the agents that run on ttal also develop ttal.

The researcher investigates features. The designer writes implementation plans. Workers implement them. Reviewers check the code. Every improvement to ttal makes the agents more capable, which makes them better at improving ttal further.

This isn't theoretical. The `docs/plans/` directory is full of design documents written by the design agent. The heartbeat scheduler, the PR review pipeline, the task enrichment hooks — all designed by agents, implemented by agents, reviewed by agents.

This creates a concrete constraint on design: ttal must be ergonomic for both humans and agents. Commands need to be callable by scripts, not just people. Output needs to be parseable. The CLI is the API.

```bash
# An agent running ttal to spawn another agent's work
ttal task execute a1b2c3d4

# An agent querying its own task queue
ttal task find refactor --completed
```

The goal is a system where improving the tool and using the tool are the same activity.

---

## 4. Agent-Native Design

ttal treats AI agents as first-class team members, not disposable processes.

**Persistent identity** — agents have names, roles, personalities, and voices. Not `worker-1` and `worker-2`. Your researcher is Athena. Your orchestrator is Kestrel. These names appear in logs, messages, and PR comments. Identity creates accountability.

**Per-agent Telegram bots** — each agent is its own chat. You talk to your researcher about research. You talk to your designer about plans. Not everything routed through one bot thread.

**Agent-to-agent messaging** — horizontal communication without going through a human. The designer can ask the researcher for clarification directly via `ttal send`. The reviewer can notify the coder that a PR is ready.

**Memory and continuity** — agents have `CLAUDE.md` files, memory directories, diary entries. They build context across sessions. A researcher who's investigated your codebase once doesn't start from zero next time.

The alternative — treating agents as stateless functions — loses everything that makes them valuable beyond a single invocation. ttal's agents are team members who accumulate knowledge about the project, the codebase, and each other.

---

## 5. Config Over Code

ttal pushes behavior into configuration, not source code.

**Prompts in `config.toml`** — the execute prompt, review prompt, and triage prompt are all configurable fields. Change how workers behave without rebuilding the binary.

**Roles in `roles.toml`** — per-role tags, routing rules, prompt templates. Add a new role by editing TOML. Remove one the same way.

**Skills as files** — agent capabilities are markdown files deployed via `ttal sync`. Drop a file into `~/clawd/docs/skills/`, sync, and every agent gains that capability.

**Project store as TOML** — add a project with `ttal project add alias name /path`. Remove it with `ttal project remove`. No database, no migrations.

The principle: the people who use ttal daily — agents and their human — should be able to change behavior without touching Go code. The binary is plumbing. The config is the product. When an agent needs a new capability, the answer is a new skill file, not a pull request to the CLI.
