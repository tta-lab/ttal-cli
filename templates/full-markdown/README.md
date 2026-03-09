---
name: Full (Markdown)
description: Complete team with plans stored as markdown in your repo
agents: manager, designer, researcher, debugger
---

# Full (Markdown) — 4-Agent Team

A complete ttal team with task management, research, design, and bug diagnosis. Plans and research are stored as markdown files in your project repos.

## Agents

- **manager** — task orchestrator. Routes work, manages priorities, spawns workers.
- **researcher** — investigator. Explores codebases, reads docs, writes structured findings.
- **designer** — architect. Turns research into step-by-step implementation plans.
- **debugger** — bug fix designer. Diagnoses bugs and writes fix plans for workers to execute.

## The pipeline

```
task add "Build feature X" +research
  → researcher investigates, writes findings to docs/research/
  → task moves to +design
  → designer writes plan to docs/plans/
  → you approve
  → manager spawns a worker: ttal task execute <uuid>
  → worker executes, creates PR
  → reviewer checks PR, worker triages feedback

Bug reported → debugger diagnoses, writes fix plan → you approve → manager spawns worker
```

## Quick start

1. Set up your workspace:
   ```bash
   ttal onboard --scaffold full-markdown
   ```
2. Edit `~/.config/ttal/config.toml`:
   - Set `chat_id` and `team_path`
3. Create 4 Telegram bots via @BotFather
4. Add tokens to `~/.config/ttal/.env`:
   ```
   MANAGER_BOT_TOKEN=...
   RESEARCHER_BOT_TOKEN=...
   DESIGNER_BOT_TOKEN=...
   DEBUGGER_BOT_TOKEN=...
   DEFAULT_NOTIFICATION_BOT_TOKEN=...   # Team notifications (worker status, PR events)
   ```
5. Run `ttal doctor` to verify
6. Start the daemon: `ttal daemon start`
