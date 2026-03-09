---
name: setup
description: First-time ttal setup. Installs ttal, then runs ttal onboard for daemon, hooks, and config. Run this after cloning a ttal workspace template.
---

# TTAL Setup

Run setup steps automatically. Only pause when user action is required (Telegram bot creation, pasting tokens). Fix things yourself when possible.

**Principle:** Do the work. Only ask the user when you genuinely need their input (secrets, choices).

**UX Note:** Use `AskUserQuestion` for all user-facing questions.

## Context

This skill runs inside the ttal-cli repository. All documentation is available
in `docs/` (VitePress site) and templates are in `templates/`. When the user asks
questions about ttal, read from `docs/` directly — no network requests needed.

## 1. Install ttal

Check if ttal is installed:

```bash
which ttal && echo "TTAL_OK=true" || echo "TTAL_OK=false"
```

If TTAL_OK=false, detect platform and install:
- macOS: `brew tap tta-lab/ttal && brew install ttal`
- Linux/other: `go install github.com/tta-lab/ttal-cli@latest`
  - If `go` not found: `brew install go` (macOS) or guide user to install Go

Verify: `ttal version`

## 2. Run ttal onboard

`ttal onboard` handles everything: prerequisites (tmux, taskwarrior, ffmpeg), workspace scaffold, taskwarrior UDAs, daemon install, worker hooks. When it runs `ttal init`, templates are found locally in this repo — no network clone needed. Just run it:

```bash
ttal onboard
```

If onboard reports fixable issues, re-run with `ttal onboard --fix`. Parse its output for any errors that need user attention.

After onboard completes, capture workspace path and verify team_path points to current directory:

```bash
WORKSPACE=$(pwd)
grep "team_path" ~/.config/ttal/config.toml
```

If team_path doesn't match current workspace, update it using `perl -i -pe` (portable across macOS and Linux):

```bash
perl -i -pe "s|team_path = .*|team_path = \"$WORKSPACE\"|" ~/.config/ttal/config.toml
```

## 3. Telegram Bot Setup (Optional)

AskUserQuestion: "Would you like to set up Telegram integration? This lets you manage agents from your phone. (You can skip this and add it later.)"

If yes:

### 3a. Get chat_id

Tell user:
1. Open Telegram, search for `@userinfobot`
2. Send `/start` — it replies with your chat ID
3. Ask user to paste their chat ID

Write the chat_id to config.toml under `[teams.default]`:

```bash
perl -i -pe "s|chat_id = .*|chat_id = \"<pasted-id>\"|" ~/.config/ttal/config.toml
```

If the `chat_id` key doesn't exist yet, append it under `[teams.default]`:

```bash
perl -i -pe 's/(\[teams\.default\])/\1\nchat_id = "<pasted-id>"/' ~/.config/ttal/config.toml
```

### 3b. Create agent bots

Discover agents in the workspace root (`$WORKSPACE`):

```bash
shopt -s nullglob
for dir in */CLAUDE.md; do dirname "$dir"; done
```

Token naming convention: uppercase the agent name and append `_BOT_TOKEN`. For example: agent `manager` → `MANAGER_BOT_TOKEN`, agent `yuki` → `YUKI_BOT_TOKEN`.

For each agent, tell user:
1. Open Telegram, search for `@BotFather`
2. Send `/newbot`
3. Name: `<agent-name> (ttal)` (or whatever they want)
4. Username: `<agent-name>_ttal_bot` (must end in `bot`)
5. Copy the bot token

After user creates all bots, ask them to paste each token. Write to `~/.config/ttal/.env` using heredoc to avoid token exposure in process list:

```bash
# For each agent (example for agent named "yuki"):
cat >> ~/.config/ttal/.env << 'ENVEOF'
YUKI_BOT_TOKEN=<pasted-token>
ENVEOF
```

### 3c. Notification bot

Tell user to create one more bot for system notifications (PR status, CI results):
- Suggested username: `ttal_notify_bot`

Write token as `DEFAULT_NOTIFICATION_BOT_TOKEN` to `.env`:

```bash
cat >> ~/.config/ttal/.env << 'ENVEOF'
DEFAULT_NOTIFICATION_BOT_TOKEN=<pasted-token>
ENVEOF
```

### 3d. Verify

```bash
ttal daemon restart   # pick up new tokens
ttal doctor           # should show all green
```

## 4. Verify & Done

Run final health check:

```bash
ttal doctor
```

If all green, print success with discovered agents and next steps:

```
✓ ttal is ready!

Your agents:
  (list discovered agent directories)

Next steps:
  • Start your agents: ttal team start
  • Add a project: ttal project add myapp --path /path/to/repo
  • Create a task: ttal task add --project myapp "Build the thing"
```

If warnings/errors remain, explain each and offer to fix.
