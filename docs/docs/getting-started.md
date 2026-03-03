---
title: Getting Started
description: Install ttal and set up your first agent team
---

## Installation

### Homebrew (macOS/Linux)

```bash
brew tap tta-lab/ttal
brew install ttal
```

### Via `go install`

```bash
go install github.com/tta-lab/ttal-cli@latest
```

### From source

```bash
git clone https://github.com/tta-lab/ttal-cli.git
cd ttal-cli
make install
```

### Prerequisites

ttal requires these tools in your `$PATH`:

- **Go 1.22+** — for building from source
- **tmux** — terminal multiplexer for agent sessions
- **git** — version control and worktree management
- **taskwarrior** — task tracking (`task` command)

## Initial setup

### Quick setup with ttal init

The fastest way to get started — pick a scaffold and go:

```bash
# See available scaffolds interactively
ttal init

# Or specify directly
ttal init --scaffold basic              # 2 agents: manager, designer
ttal init --scaffold full-markdown      # 4 agents: manager, researcher, designer, lifecycle
ttal init --scaffold full-flicknote     # 4 agents with FlickNote integration
```

This clones a starter template, copies it to `~/ttal-workspace`, and installs a config file.

### Full onboarding

For a guided setup that also installs prerequisites and the daemon:

```bash
# Default: basic scaffold
ttal onboard

# With a specific scaffold
ttal onboard --scaffold full-markdown --workspace ~/my-agents
```

Onboarding walks through:
1. Install prerequisites via brew (tmux, taskwarrior, zellij, ffmpeg)
2. Set up workspace from a scaffold template
3. Set up taskwarrior UDAs and config template
4. Register discovered agents in the database
5. Install daemon launchd plist and taskwarrior hooks

### After init or onboard

1. **Edit config** — `~/.config/ttal/config.toml`: set `chat_id` and `team_path`
2. **Create Telegram bots** via @BotFather (one per agent)
3. **Add bot tokens** to `~/.config/ttal/.env`
4. **Verify**: `ttal doctor`
5. **Start daemon**: `ttal daemon start`

### Register a project

```bash
ttal project add myapp --path=/path/to/project +backend
```

Agents with matching tags can see and work on this project.

### Start the daemon

```bash
ttal daemon status   # Check if it's running
```

If the daemon isn't running, launchd should start it automatically after `ttal daemon install`. For debugging, you can run it in the foreground:

```bash
ttal daemon
```

## What's next

- [Configuration](/docs/configuration) — config file reference
- [Agents](/docs/agents) — agent management and identity
- [Messaging](/docs/messaging) — Telegram bridge setup
- [Tasks](/docs/tasks) — task-driven workflows
