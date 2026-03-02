---
title: Getting Started
description: Install ttal and set up your first agent team
---

## Installation

### From source

```bash
git clone https://github.com/tta-lab/ttal-cli.git
cd ttal-cli
make install
```

### Via `go install`

```bash
go install github.com/tta-lab/ttal-cli@latest
```

### Prerequisites

ttal requires these tools in your `$PATH`:

- **Go 1.22+** — for building from source
- **tmux** — terminal multiplexer for agent sessions
- **git** — version control and worktree management
- **taskwarrior** — task tracking (`task` command)

## Initial setup

### 1. Install taskwarrior hooks

ttal uses taskwarrior hooks to enrich tasks and route them to agents automatically.

```bash
ttal worker install
```

This installs two hooks: `on-add-ttal` and `on-modify-ttal` in your taskwarrior hooks directory.

### 2. Install the daemon

The daemon is a long-running process that handles Telegram messaging, worker cleanup, and agent communication.

```bash
ttal daemon install
```

On macOS, this creates a launchd plist and a config template at `~/.config/ttal/config.toml`.

### 3. Run onboarding

The guided onboarding walks you through initial configuration:

```bash
ttal onboard
```

This helps you set up your config file and team settings. Bot tokens are stored separately in `~/.config/ttal/.env`.

### 4. Register your first agent

```bash
ttal agent add kestrel +core +backend
```

Verify it's registered:

```bash
ttal agent list
```

You should see your agent listed with its tags.

### 5. Register a project

```bash
ttal project add myapp --path=/path/to/project +backend
```

Because both `kestrel` and `myapp` share the `+backend` tag, kestrel can see and work on this project.

### 6. Start the daemon

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
