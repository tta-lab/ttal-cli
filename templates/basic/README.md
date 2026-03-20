---
name: Basic
description: Get started with task management and planning
agents: manager, designer
---

# Basic — Manager + Designer

A minimal ttal setup with two agents: one manages tasks, one designs implementation plans. Workers (coders and reviewers) are spawned on-demand.

## Agents

- **manager** — task orchestrator. Manages taskwarrior tasks, routes work, handles daily focus.
- **designer** — context enricher. Reads the codebase and writes implementation plans so workers can execute without guessing.

## The core loop

```
You create a task
  → manager advances it: ttal task go <uuid>
  → designer writes an implementation plan
  → you approve
  → manager spawns a worker: ttal task go <uuid>
  → worker executes the plan, creates a PR
```

## Quick start

1. Set up your workspace:
   ```bash
   ttal onboard --scaffold basic
   ```
2. Edit `~/.config/ttal/config.toml`:
   - Set `chat_id` (your Telegram chat ID — get from @userinfobot)
   - Set `team_path` (path to your workspace, e.g. `~/ttal-workspace`)
3. Create Telegram bots via @BotFather (one per agent)
4. Add bot tokens to `~/.config/ttal/.env`:
   ```
   MANAGER_BOT_TOKEN=123:ABC...
   DESIGNER_BOT_TOKEN=456:DEF...
   DEFAULT_NOTIFICATION_BOT_TOKEN=789:GHI...   # Team notifications (worker status, PR events)
   ```
5. Run `ttal doctor` to verify
6. Start the daemon: `ttal daemon start`

## Adding more agents

Create a new directory with a `CLAUDE.md` file, then register it:

```bash
mkdir researcher
# Write researcher/CLAUDE.md
ttal agent add researcher +core
```

Consider upgrading to the `full-markdown` or `full-flicknote` scaffold when you need more agents.
