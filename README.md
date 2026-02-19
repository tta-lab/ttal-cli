# TTAL CLI - Agent Infrastructure

A command-line tool for managing agents, projects, workers, PRs, messaging, and voice — the single interface for agent operations in zellij sessions.

## Features

- **Project Management**: Add, list, archive, and tag projects
- **Agent Management**: Configure agents with tags, status tracking, and heartbeat periods
- **Worker Management**: Spawn, close, and poll Claude Code workers in isolated zellij sessions
- **PR Management**: Create, modify, merge (squash), and comment on Forgejo PRs — context auto-resolved from worker session
- **Messaging**: Bidirectional agent ↔ human (Telegram) and agent ↔ agent (Zellij) communication via `ttal send`
- **Tag-Based Routing**: Tag-based task routing to matching agents on task start
- **Memory Capture**: Extract git commits and generate agent-filtered memory logs
- **Voice**: Text-to-speech using per-agent Kokoro voices on Apple Silicon
- **Daemon**: Communication hub — Telegram polling, message delivery, worker completion polling (launchd)

## Installation

```bash
# Build and install binary
make install

# Set up taskwarrior hook (routes task events to agents)
ttal worker install

# Set up daemon (Telegram integration + completion polling, macOS)
ttal daemon install
```

To remove:
```bash
ttal worker uninstall
ttal daemon uninstall
```

## Development

### Quick Start

```bash
# Format code
make fmt

# Generate ent code from schemas
make generate

# Run tests
make test

# Build binary
make build

# Run all checks (CI equivalent)
make ci
```

### Pre-commit Hooks

Install git hooks to automatically run checks before each commit:

```bash
make install-hooks
```

The pre-commit hook runs:
- Code formatting (`make fmt`)
- Ent code generation (`make generate`)
- Vet checks (`make vet`)
- Tests (`make test`)

### Code Quality

This project uses:
- **gofmt** - Code formatting
- **golangci-lint** - Comprehensive linting
- **go vet** - Static analysis
- **ent** - Schema-first database with auto-generated type-safe queries

Run linting:
```bash
make lint
```

## CI/CD

### Workflows

**PR Workflow** (`.forgejo/workflows/pr.yaml`)
- Runs on all pull requests
- Checks formatting, vet, linting
- Verifies ent generated code is up-to-date
- Runs tests and builds binary

**CI Workflow** (`.forgejo/workflows/ci.yaml`)
- Runs on push to main
- Full build and lint checks
- Ensures main branch stays healthy

**Release Workflow** (`.forgejo/workflows/release.yaml`)
- Triggers on version tags (e.g., `v1.0.0`)
- Builds binaries for multiple platforms:
  - Linux (amd64, arm64)
  - macOS (amd64, arm64)
  - Windows (amd64)
- Creates GitHub/Forgejo release with binaries

### Creating a Release

```bash
# Tag a new version
git tag v1.0.0
git push origin v1.0.0

# Release workflow automatically builds and publishes
```

## Database Schema

The CLI uses SQLite with four tables:

1. **projects**: Project information and metadata
2. **agents**: Agent configuration and status
3. **project_tags**: Many-to-many relationship for project tags
4. **agent_tags**: Many-to-many relationship for agent tags

## Usage

### Project Commands

```bash
# Add a project
ttal project add --alias=clawd --name='TTAL Core' --path=/Users/neil/clawd --repo=neil/clawd --repo-type=forgejo +infrastructure +core

# List all projects
ttal project list

# List projects with specific tags
ttal project list +backend +infrastructure

# Get project details
ttal project get clawd

# Modify project tags (use -- to separate from flags)
ttal project modify clawd -- +new-tag -old-tag

# Modify project fields
ttal project modify clawd -- name:'New Project Name'
ttal project modify clawd -- path:/new/path
ttal project modify clawd -- description:'Updated description'
ttal project modify clawd -- repo:owner/repo-name
ttal project modify clawd -- repo-type:forgejo
ttal project modify clawd -- owner:username

# Modify multiple fields and tags at once
ttal project modify clawd -- name:'New Name' path:/new/path +backend -legacy

# Archive a project
ttal project archive old-project

# Unarchive a project
ttal project unarchive old-project
```

### Agent Commands

```bash
# Add an agent
ttal agent add yuki --path=/Users/neil/clawd/.openclaw-main --heartbeat=120 +secretary +core

# List all agents
ttal agent list

# List agents with specific tags
ttal agent list +research

# Get agent info (includes matching projects)
ttal agent info yuki

# Modify agent tags (use -- to separate from flags)
ttal agent modify yuki -- +new-tag -old-tag

# Modify agent path
ttal agent modify yuki -- path:/Users/neil/clawd/.openclaw-main

# Modify path and tags together
ttal agent modify yuki -- path:/new/path +backend -legacy

# Update agent status
ttal agent status yuki busy

# Update heartbeat period
ttal agent heartbeat yuki 120
```

### Memory Capture

```bash
# Capture memory for today
ttal memory capture

# Capture memory for a specific date
ttal memory capture --date=2026-02-08

# Specify output directory
ttal memory capture --date=2026-02-08 --output=/path/to/memory
```

### Worker Commands

Worker commands manage Claude Code instances running in isolated zellij sessions, tracked via taskwarrior. These commands do **not** require the ttal database.

```bash
# List active workers
ttal worker list

# Spawn a new worker
ttal worker spawn --name fix-auth --project ~/code/myapp --task <uuid>

# Spawn with brainstorming mode (explores design before implementing)
ttal worker spawn --name design-api --project ~/code/myapp --task <uuid> --brainstorm

# Spawn without worktree (work directly in project directory)
ttal worker spawn --name hotfix --project ~/code/myapp --task <uuid> --worktree=false

# Force respawn (close existing session)
ttal worker spawn --name fix-auth --project ~/code/myapp --task <uuid> --force

# Close a worker (smart mode - auto-cleanup if PR merged + clean worktree)
ttal worker close <session-name>

# Force close (dump state and cleanup regardless of PR status)
ttal worker close <session-name> --force

# Poll for completed workers (auto-completes tasks with merged PRs)
ttal worker poll
```

#### Spawn Flags

| Flag | Default | Description |
|------|---------|-------------|
| `--name` | (required) | Worker name, used for branch and worktree naming |
| `--project` | (required) | Project directory path |
| `--task` | (required) | Taskwarrior task UUID |
| `--session` | random 8-char ID | Custom zellij session name |
| `--worktree` | `true` | Create git worktree for isolation |
| `--force` | `false` | Force respawn (close existing session) |
| `--yolo` | `true` | Skip Claude Code permission prompts |
| `--brainstorm` | `false` | Use brainstorming skill before implementation |

#### Close Exit Codes

| Code | Meaning |
|------|---------|
| 0 | Cleaned up successfully |
| 1 | Needs manual decision (PR not merged, dirty worktree) |
| 2 | Error (worker not found, script failure) |

#### Worker Setup

`ttal worker install` installs two taskwarrior hooks (`on-add-ttal` and `on-modify-ttal`).
Worker completion polling is handled by the daemon (see [Daemon Setup](#daemon-setup) below).

```bash
# Install taskwarrior hooks
ttal worker install

# Remove taskwarrior hooks
ttal worker uninstall

# View hook logs
tail -f ~/.task/hooks.log
```

#### Task Lifecycle

Tasks go through three hook-driven stages:

**1. on-add: Auto-enrichment** — When a task is created, the on-add hook checks if its tags already match a registered agent. If not, it forks a background `claude -p --model haiku` process to enrich the task with `project_path` and `branch` UDAs.

**2. on-modify (start): Deterministic spawn** — When a task is started (`task <id> start`), the on-modify hook reads the enriched UDAs and forks a background `ttal worker spawn` to create a zellij session with a git worktree.

**3. on-modify (complete): Auto-cleanup** — When a task is completed, the hook closes the worker session, auto-cleans if the PR is merged and the worktree is clean, or notifies the lifecycle agent if manual cleanup is needed.

All lifecycle events are reported to the lifecycle agent's Telegram chat via the daemon socket.

```bash
# Example flow:
task add "Fix auth timeout in login API" +backend    # on-add: haiku enriches with project_path/branch
task <id> start                                       # on-modify: spawns worker/fix-auth-timeout
task <id> done                                        # on-modify: auto-cleanup if PR merged
```

If a task's tags already match an agent (e.g., `+kestrel`), enrichment is skipped — the task is assumed to be pre-configured.

### Daemon Setup

The daemon is a long-running process (managed by launchd on macOS) that acts as a communication hub for agents.

#### What it does

- **Telegram → Agent**: Polls each agent's Telegram bot for inbound messages, delivers them to the agent's zellij session via `write-chars`
- **CC → Telegram**: JSONL watcher tails active CC session files (via fsnotify) and sends assistant text blocks to Telegram automatically — no agent action needed
- **Agent → Agent**: Routes `ttal send --from a --to b` between agents via zellij with attribution
- **Worker completion polling**: Checks for merged PRs every 60 seconds and auto-completes taskwarrior tasks

#### Config

Create `~/.ttal/daemon.json` (a template is created by `ttal daemon install`):

```json
{
  "zellij_session": "ttal-team",
  "chat_id": "845849177",
  "lifecycle_agent": "kestrel",
  "agents": {
    "kestrel": {
      "bot_token": "123:ABC..."
    }
  }
}
```

- `zellij_session` — all agents live in this session (tab name = agent name)
- `chat_id` — global Telegram chat ID (per-agent override via `chat_id` field)
- `lifecycle_agent` — which agent handles task start/complete events
- `agents` — per-agent Telegram bot tokens

#### Commands

```bash
# Install launchd plist + create config template
ttal daemon install

# Remove launchd plist, socket, and pid file
ttal daemon uninstall

# Check if daemon is running
ttal daemon status

# Fill bot tokens from environment variables
ttal daemon sync-tokens

# Run daemon in foreground (for debugging)
ttal daemon
```

#### Logs

```bash
tail -f ~/.ttal/daemon.log
```

### Messaging — `ttal send`

Send messages between agents and humans with explicit direction:

```bash
# System/hook delivers to agent via Zellij
ttal send --to kestrel "Task started: implement auth"

# Agent-to-agent via Zellij (recipient sees [agent from:yuki] attribution)
ttal send --from yuki --to kestrel "Can you review my auth module?"

# Read message from stdin
echo "done" | ttal send --to kestrel --stdin
```

> **Note:** Agent → Telegram is handled automatically by the daemon's JSONL watcher — agents don't need to call `ttal send` to reach humans.

Message formats delivered to CC terminal:

```
[telegram from:neil]
Can you check the deployment?

[agent from:yuki]
Can you review my auth module?
```

### PR Commands

Manage Forgejo pull requests directly from worker sessions. Context is auto-resolved from `ZELLIJ_SESSION_NAME` → task `session_name` UDA → `project_path` → `git remote get-url origin`.

```bash
# Create PR (stores pr_id in task UDA automatically)
ttal pr create "feat: add user authentication"
ttal pr create "fix: timeout bug" --body "Fixes #42"

# Modify PR title or body
ttal pr modify --title "updated title"
ttal pr modify --body "updated description"

# Squash-merge the PR (deletes branch by default)
ttal pr merge
ttal pr merge --keep-branch

# Add a comment to the PR
ttal pr comment create "LGTM, ready to merge"

# List comments on the PR
ttal pr comment list

# Override auto-resolution with explicit task UUID
ttal pr create "title" --task <uuid>
```

Requires `FORGEJO_URL` and `FORGEJO_TOKEN` environment variables.

## Environment Variables

### Forgejo API

| Variable | Required | Description |
|----------|----------|-------------|
| `FORGEJO_URL` | Yes | Forgejo instance URL (e.g., `https://git.guion.io`) |
| `FORGEJO_TOKEN` or `FORGEJO_ACCESS_TOKEN` | Yes | Forgejo API token |

Required by: `ttal pr *`, `ttal worker poll`, `ttal worker close` (smart mode).

### Worker Commands

| Variable | Required | Description |
|----------|----------|-------------|
| `TTAL_ZELLIJ_DATA_DIR` | No | Custom zellij data directory (default: `$TMPDIR/ttal-zellij-data`) |

### Taskwarrior UDAs

Worker commands require these User Defined Attributes in `~/.taskrc`:

```
uda.session_name.type=string
uda.session_name.label=Session Name

uda.branch.type=string
uda.branch.label=Branch

uda.project_path.type=string
uda.project_path.label=Project Path

uda.pr_id.type=string
uda.pr_id.label=PR ID
```

### External Dependencies

Worker commands require these tools in `$PATH`:

- `task` - Taskwarrior (task tracking)
- `zellij` - Terminal multiplexer (worker sessions)
- `git` - Version control (worktrees, branch management)
- `fish` - Fish shell (used in zellij layouts)

## Modify Command Syntax

The `modify` command supports both tag operations and field updates using taskwarrior-like syntax:

### Tag Operations
- `+tag` - Add a tag
- `-tag` - Remove a tag
- `+tag1 +tag2` - Add multiple tags
- `-old +new` - Mix add and remove operations

### Field Updates
- `field:value` - Update a field value
- Use quotes for values with spaces: `name:'My Project Name'`
- Combine multiple operations: `path:/new/path +backend -legacy`

### Available Fields

**Agent fields:**
- `path` - Agent workspace path

**Project fields:**
- `name` - Project name
- `description` - Project description
- `path` - Filesystem path
- `repo` - Repository ID (e.g., owner/repo)
- `repo-type` - Repository type (forgejo, github, or codeberg)
- `owner` - Project owner

### Important Notes
- Always use `--` before your modifications to prevent `-tag` from being interpreted as a command flag
- Field names are case-insensitive
- Tag names are automatically converted to lowercase

### Examples
```bash
# Tags only
ttal project modify clawd -- +backend -legacy

# Fields only
ttal project modify clawd -- name:'New Name' path:/new/path

# Combined
ttal agent modify yuki -- path:/new/path +research -demo
```

## Tag System

Tags use taskwarrior-like syntax as described above in the Modify Command Syntax section

### Agent-Project Matching

Agents see projects that share at least one tag. Tags also drive task routing — when a taskwarrior task is started, it's routed to the agent with the most matching tags.

```bash
# Agent with tags: research, design
ttal agent add athena +research +design

# Project with tags: core, infrastructure
ttal project add clawd +core +infrastructure

# athena can work on projects with research or design tags
ttal agent info athena  # Shows matching projects

# Tasks tagged +research are automatically routed to athena
# Tasks with no matching agent go to worker-lifecycle (default)
```

## Memory File Format

Memory files are generated in markdown format:

```markdown
# Memory Log: yuki - 2026-02-08

Generated: 2026-02-08 14:30:00
Total commits: 15

## Project: clawd

### Category: impl

- `a1b2c3d` Add project manager schema (14:30)
- `e4f5g6h` Implement CLI commands (15:45)

### Category: refactor

- `i7j8k9l` Optimize query performance (16:20)
```

## Database Location

By default, the database is stored at `~/.ttal/ttal.db`. You can specify a custom location:

```bash
ttal --db=/custom/path/ttal.db project list
```

## Commit Convention

When building ttal-cli, use the following commit format:

```
ttal: [category] description

Example: ttal: impl - project schema + sqlite
         ttal: refactor - query optimization
```

## Status Values

Agents can have one of three status values:

- `idle` - Available for task assignment
- `busy` - Currently working
- `paused` - Temporarily disabled

## License

MIT
