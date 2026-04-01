# TTal

Agent ops for multi-repo teams. One binary. CLI-native.

Most agent tools assume one repo, one session, one task. Real projects span multiple repos, multiple languages, multiple deployment targets. TTal coordinates agents across all of them — routing tasks, spawning parallel workers, shipping PRs — while you manage everything from Telegram.

```
task → research → design → implement → review → merge → cleanup
```

![Yuki Chat](docs/public/screenshots/yuki-chat.jpg)

## What it does

- **Route tasks** to the right agent — researcher investigates, designer plans, worker implements
- **Spawn workers** in isolated git worktrees — parallel execution across repos, zero conflicts
- **Ship PRs** with automated review — specialized reviewers for security, tests, types, edge cases
- **Manage from Telegram** — approve, merge, redirect, all from your phone
- **One command drives everything** — `ttal go` advances any task through its pipeline

```bash
# Create a task
ttal task add --project myapp "Add JWT authentication to the API"

# One command drives every transition
ttal go abc12345

# Pipeline routes it: research → design → implement → review → merge
# You review verdicts on Telegram, approve, done
```

## Multi-repo, not monorepo

Multi-repo coordination is a [known pain point](https://github.com/anthropics/claude-code/issues/23627) — Claude Code assumes one session per repo, and there's no native way to plan across repos, share context between sessions, or coordinate branches.

TTal solves this with a project registry and a coordination layer:

```toml
# ~/.config/ttal/projects.toml
[ttal]
name = "TTal Core"
path = "/code/ttal-cli"

[organon]
name = "Organon"
path = "/code/organon"

[temenos]
name = "Temenos"
path = "/code/temenos"
```

Register your projects once. TTal handles the rest:

- **Cross-repo context** — `ei ask --project organon "how does src handle markdown?"` spawns a sandboxed agent in any project. Research without polluting your main session.
- **Session forking** — brainstorm a feature in one session, fork it into project-specific planning sessions. Each fork carries the full conversation — every decision, every "actually, let's not do that" — with zero context loss.
- **Parallel workers across repos** — a feature that touches three repos gets three workers, each in their own git worktree, each sandboxed to their project. They can't see or step on each other.
- **CI integration** — workers subscribe to check status. When CI fails, the daemon delivers the log directly to the worker's session. They read it, fix it, push again. No human in the loop for lint and test failures.

A cross-repo feature that touches TTal, temenos, and organon gets three parallel workers, three PRs, three review cycles — all coordinated through one pipeline.

## How it works

Two planes, borrowed from networking's control/data plane separation:

**Manager plane** — persistent agents that hold the big picture. They know what features they designed with you, which tasks are blocked, what shipped yesterday. Managers never touch code.

**Worker plane** — ephemeral sessions that implement. Each gets its own git worktree, sandboxed environment, and tmux session. Spin up, do the work, merge, clean up. Workers never worry about the big picture.

**Message bridge** — Human ↔ agent via Telegram. Agent ↔ agent via `ttal send`. CI status, PR reviews, task updates — all routed through a single daemon. You talk to your agents like coworkers in a group chat.

```
┌─────────────────────────────────────────┐
│  TTal         orchestration layer       │
│               tasks, workers, pipeline  │
├─────────────────────────────────────────┤
│  organon      instruments               │
│               src, web (structure-aware)│
├─────────────────────────────────────────┤
│  logos        reasoning engine          │
│               bash-only agent loop      │
│               any LLM, no tool schemas  │
├─────────────────────────────────────────┤
│  CC sandbox   the sacred boundary       │
│               seatbelt / bwrap          │
│               OS-native, no containers  │
└─────────────────────────────────────────┘
```

Each layer does one thing. **TTal** orchestrates. **[organon](https://github.com/tta-lab/organon)** perceives and edits — structure-aware, not text-aware. **[logos](https://github.com/tta-lab/logos)** thinks — bash-only reasoning, works with any LLM. CC's native sandbox isolates — no containers needed.

## The team

TTal agents aren't chatbots. They're specialists with clear roles:

| Agent | Role | What they do |
|-------|------|--------------|
| Yuki 🐱 | Orchestrator | Routes tasks, manages the pipeline |
| Athena 🦉 | Researcher | Investigates problems, writes findings |
| Inke 🐙 | Designer | Reads research, writes implementation plans |
| Workers | Coders | Spawn per-task, implement, open PRs, self-cleanup |

Each agent runs in its own tmux session. Workers get isolated git worktrees. The daemon handles all messaging.

![Telegram Chat](docs/public/screenshots/telegram.png)

## Built with TTal

We built TTal with TTal. 476 PRs merged, 42k lines of Go — last 30 days. Then we pointed it at flicknote-cli — 55 PRs merged in 15 days, Rust. Same pipeline, different repo, same velocity.

![Heatmap](docs/public/screenshots/heatmap.png)

## Install

```bash
brew tap tta-lab/ttal
brew install ttal
```

Or from source:

```bash
go install github.com/tta-lab/ttal-cli@latest
```

## Quick start

```bash
git clone https://github.com/tta-lab/ttal-cli.git && cd ttal-cli
# Open in Claude Code, then: /setup
```

The setup skill installs TTal, configures hooks, and walks you through Telegram integration. Five minutes to your first automated PR.

Or manually:

```bash
ttal doctor --fix      # install hooks
ttal daemon install    # start the communication hub
ttal sync              # generate sandbox config from project registry
```

## Open source, sustainably

TTal is MIT licensed. The code is yours — fork it, read it, learn from it, remove anything you want.

A license key unlocks unlimited agents. Without one, you get 2 agents — enough to try the pipeline end-to-end and see if TTal fits your workflow. Pro is $100/year, or you can earn it: every merged PR to a TTal ecosystem repo gets you 1 month of pro for free. Contribute code, earn your seat.

This isn't a restriction disguised as open source. It's a value exchange. The gate is in the code, clearly marked, easy to find. MIT means you have every right to remove it. We're betting that if TTal saves you time, $100/year is worth not bothering.

The PR rewards aren't charity either — contributors become power users, power users find real problems, real problems become good PRs. That's the flywheel we want.

## Design principles

- **Unix philosophy.** Task management via Taskwarrior, knowledge via FlickNote, editing via tree-sitter. Compose dedicated tools, don't bundle into a platform.
- **Structure-aware, not text-aware.** Files have symbols. Web pages have headings. Notes have sections. Every tool targets by ID, not by reproducing text.
- **Isolation by default.** Workers get sandboxes and worktrees. Parallel execution requires it.
- **CLI-native.** Every tool is a stateless CLI command. Agents use them the same way humans would.
- **Human not in the loop — until the loop needs a human.** Agents self-correct via CI, pre-commit hooks, and review loops. You step in when judgement calls matter.

## Ecosystem

TTal is the execution layer of the [tta-lab](https://github.com/tta-lab) ecosystem:

| Tool | What it does |
|------|-------------|
| **[TTal](https://github.com/tta-lab/ttal-cli)** | Agent orchestration — tasks, workers, pipelines |
| **[organon](https://github.com/tta-lab/organon)** | Structure-aware reading and editing — `src` and `web` |
| **[logos](https://github.com/tta-lab/logos)** | Bash-only reasoning engine — any LLM, no tool schemas |
| **[FlickNote](https://flicknote.app/)** | Knowledge capture — voice memos, links, plans, agent memory |

No MCP for core operations. CLI-first. Agents use the same tools you do.

## Further reading

- [TTal — More Than a Harness Engineering Framework](https://dev.to/neil_agentic/ttal-more-than-a-harness-engineering-framework-2pbn) — the philosophy
- [We Replaced Every Tool Claude Code Ships With](https://dev.to/neil_agentic/we-replaced-every-tool-claude-code-ships-with-522j) — the tooling
- [Managing 15+ Repos with Claude Code](https://dev.to/neil_agentic/managing-15-repos-with-claude-code-via-a-coordination-layer-4fg4) — multi-repo workflow
- [How We Manage Memory and Sessions](https://dev.to/neil_agentic/how-we-manage-memory-and-sessions-in-a-multi-agent-claude-code-system-2a9k) — persistence model

## License

MIT — yes, really. The snail carries its house, not a lock. 🐌
