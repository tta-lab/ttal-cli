# TTAL CLI - Agent Infrastructure

A command-line tool for managing agents, projects, workers, PRs, messaging, and voice — the single interface for agent operations in tmux sessions.

## Features

- **Project Management**: Add, list, archive, and tag projects
- **Agent Management**: Configure agents with tags, status tracking, and heartbeat periods
- **Worker Management**: Spawn and close Claude Code workers in isolated sessions
- **PR Management**: Create, modify, merge (squash), and comment on Forgejo PRs — context auto-resolved from worker session
- **Messaging**: Bidirectional agent ↔ human (Telegram) and agent ↔ agent (tmux) communication via `ttal send`
- **Tag-Based Routing**: Tag-based task routing to matching agents on task start
- **Today Focus**: Manage daily task focus list via taskwarrior's `scheduled` date
- **Task Routing**: Route tasks to design/research/test agents or spawn workers with `ttal task design|research|test|execute`
- **Task Utilities**: Search tasks and export rich prompts with inlined markdown context
- **Voice**: Text-to-speech using per-agent Kokoro voices on Apple Silicon
- **Daemon**: Communication hub — Telegram polling, message delivery, worker cleanup (launchd)

## Installation

### Homebrew (macOS/Linux)

```bash
brew tap tta-lab/ttal
brew install ttal
```

### From source

```bash
go install github.com/tta-lab/ttal-cli@latest
```

### With voice dictation (macOS only)

```bash
git clone https://github.com/tta-lab/ttal-cli.git
cd ttal-cli
make install-dictate
```

### Post-install setup

```bash
# Set up taskwarrior hook (routes task events to agents)
ttal worker install

# Set up daemon (Telegram integration + worker cleanup, macOS)
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

**PR Workflow** (`.github/workflows/pr.yaml`)
- Runs on all pull requests
- Checks formatting, vet, linting
- Verifies ent generated code is up-to-date
- Runs tests and builds binary

**CI Workflow** (`.github/workflows/ci.yaml`)
- Runs on push to main
- Full build and lint checks
- Ensures main branch stays healthy

**Release Workflow** (`.github/workflows/release.yaml`)
- Triggers on version tags (e.g., `v1.0.0`)
- Uses GoReleaser to build binaries for Linux/macOS (amd64, arm64)
- Creates GitHub release with archives and checksums
- Auto-pushes Homebrew formula to `tta-lab/homebrew-ttal` tap

### Creating a Release

```bash
# Tag a new version
git tag v1.0.0
git push origin v1.0.0

# GoReleaser builds binaries, creates release, and updates Homebrew formula
# Users upgrade via: brew upgrade ttal
```

## Usage

### Project Commands

Project aliases support hierarchical matching for taskwarrior integration. When a task has `project:ttal.daemon.watcher`, ttal tries matching in order: `ttal.daemon.watcher` → `ttal.daemon` → `ttal`. This lets `ttal` act as a catch-all for all sub-projects.

```bash
# Add a project
ttal project add --alias=clawd --name='TTAL Core' --path=/Users/neil/clawd +infrastructure +core

# List all projects
ttal project list

# List projects with specific tags
ttal project list +backend +infrastructure

# Modify project tags (use -- to separate from flags)
ttal project modify clawd -- +new-tag -old-tag

# Modify project fields
ttal project modify clawd -- name:'New Project Name'
ttal project modify clawd -- path:/new/path
ttal project modify clawd -- description:'Updated description'
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

### Worker Commands

Worker commands manage Claude Code instances running in isolated tmux sessions, tracked via taskwarrior. These commands do **not** require the ttal database.

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
```

#### Spawn Flags

| Flag | Default | Description |
|------|---------|-------------|
| `--name` | (required) | Worker name, used for branch and worktree naming |
| `--project` | (required) | Project directory path |
| `--task` | (required) | Taskwarrior task UUID |
| `--session` | from task UUID | tmux session name (auto-derived) |
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
Worker cleanup after PR merge is handled by the daemon (see [Daemon Setup](#daemon-setup) below).

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

**2. on-modify (start): Deterministic spawn** — When a task is started (`task <id> start`), the on-modify hook reads the enriched UDAs and forks a background `ttal worker spawn` to create a tmux session with a git worktree.

**3. on-modify (complete): Auto-cleanup** — When a task is completed, the hook closes the worker session, auto-cleans if the PR is merged and the worktree is clean, or notifies the lifecycle agent if manual cleanup is needed.

All lifecycle events are reported to the lifecycle agent's Telegram chat via the daemon socket.

```bash
# Example flow:
task add "Fix auth timeout in login API" +backend    # on-add: haiku enriches with project_path/branch
task <id> start                                       # on-modify: spawns worker/fix-auth-timeout
task <id> done                                        # on-modify: auto-cleanup if PR merged
```

If a task's tags already match an agent (e.g., `+kestrel`), enrichment is skipped — the task is assumed to be pre-configured.

### Today Commands

Manage your daily focus list. Tasks are filtered by taskwarrior's `scheduled` date. These commands do **not** require the ttal database.

```bash
# List today's focus tasks (scheduled on or before today, sorted by urgency)
ttal today list

# Show tasks completed today
ttal today completed

# Add tasks to today's focus (accepts 8-char UUID prefix or full UUID)
ttal today add <uuid> [uuid...]

# Remove tasks from today's focus
ttal today remove <uuid> [uuid...]
```

### Task Commands

Taskwarrior query utilities for searching tasks and exporting rich prompts. These commands do **not** require the ttal database.

```bash
# Export a task as a rich prompt (inlines referenced markdown files from annotations)
ttal task get <uuid>

# Search tasks by keyword (OR logic, case-insensitive)
ttal task find <keyword> [keyword...]

# Search completed tasks
ttal task find <keyword> --completed

# Route task to team's design agent (writes implementation plan)
ttal task design <uuid>

# Route task to team's research agent (researches and writes findings)
ttal task research <uuid>

# Route task to team's test agent (integration test end-to-end)
ttal task test <uuid>

# Spawn a worker to execute a task (replaces hook-based spawning)
ttal task execute <uuid>
```

`ttal task get` is designed for piping to agents — it formats the task with description, annotations, and inlined content from annotations matching `Plan:`, `Design:`, `Doc:`, `Reference:`, or `File:` patterns.

#### Task Routing

The `design`, `research`, and `test` subcommands route tasks to named agents configured per team in `config.toml`:

```toml
[teams.default]
design_agent = "inke"       # ttal task design → inke
research_agent = "athena"   # ttal task research → athena
test_agent = "sage"         # ttal task test → sage
```

Each command sends a role-tagged message (e.g., `[task design]`) with the UUID, description, and completion instructions. The agent gets full context via `ttal task get`. If the agent isn't configured, the command shows an actionable error with the exact TOML snippet to add.

### Daemon Setup

The daemon is a long-running process (managed by launchd on macOS) that acts as a communication hub for agents.

#### What it does

- **Telegram → Agent**: Polls each agent's Telegram bot for inbound messages, delivers them to the agent's tmux session via `send-keys`
- **CC → Telegram**: JSONL watcher tails active CC session files (via fsnotify) and sends assistant text blocks to Telegram automatically — no agent action needed
- **Agent → Agent**: Routes `ttal send --to b` between agents via tmux with attribution (agent identity from `TTAL_AGENT_NAME` env)
- **Worker cleanup**: Processes post-merge cleanup requests (close session, remove worktree, mark task done)

#### Config

Config lives in `~/.config/ttal/config.toml` (created by `ttal daemon install`):

```toml
[teams.default]
chat_id = "845849177"
team_path = "/path/to/agents"

[teams.default.agents.kestrel]
# Bot token resolved from .env: KESTREL_BOT_TOKEN
```

- `chat_id` — Telegram chat ID for the team
- `agents` — per-agent config (bot tokens stored in `~/.config/ttal/.env`)
- Notification bot token: `{TEAM}_NOTIFICATION_BOT_TOKEN` in `.env`

#### Commands

```bash
# Install launchd plist + create config template
ttal daemon install

# Remove launchd plist, socket, and pid file
ttal daemon uninstall

# Check if daemon is running
ttal daemon status

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
# System/hook delivers to agent via tmux
ttal send --to kestrel "Task started: implement auth"

# Agent-to-agent via tmux (recipient sees [agent from:yuki] attribution)
# Agent identity comes from TTAL_AGENT_NAME env var
ttal send --to kestrel "Can you review my auth module?"

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

Manage Forgejo pull requests directly from worker sessions. Context is auto-resolved from `TTAL_JOB_ID` (task UUID prefix) → `project_path` → `git remote get-url origin`.

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
ttal pr comment create "LGTM — no critical issues"

# List comments on the PR
ttal pr comment list
```

#### Review Flow

The reviewer is advisory only — they post a verdict but never merge:

1. **Reviewer** examines the PR and posts a comment ending with `VERDICT: LGTM` or `VERDICT: NEEDS_WORK`
2. **Coder** triages the review. Even with LGTM, the reviewer may note non-blocking issues that should be addressed first
3. **Coder** fixes remaining issues, posts a triage update via `ttal pr comment create`
4. **Coder** merges with `ttal pr merge` once all issues are addressed — the PR is already approved

Requires `FORGEJO_URL` and `FORGEJO_TOKEN` environment variables.

## Environment Variables

### Forgejo API

| Variable | Required | Description |
|----------|----------|-------------|
| `FORGEJO_URL` | Yes | Forgejo instance URL (e.g., `https://git.guion.io`) |
| `FORGEJO_TOKEN` or `FORGEJO_ACCESS_TOKEN` | Yes | Forgejo API token |

Required by: `ttal pr *`, `ttal worker close` (smart mode).

### Worker Commands

| Variable | Required | Description |
|----------|----------|-------------|
| `TTAL_JOB_ID` | No | Task UUID prefix (set automatically in worker sessions) |

### Taskwarrior UDAs

Worker commands require these User Defined Attributes in `~/.taskrc`:

```
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
- `tmux` - Terminal multiplexer (worker and agent sessions)
- `git` - Version control (worktrees, branch management)
- `fish` - Fish shell (used in tmux sessions)

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

## Commit Convention

When building ttal-cli, use the following commit format:

```
ttal: [category] description

Example: ttal: impl - add worker spawn
         ttal: refactor - optimize tag queries
```

## Status Values

Agents can have one of three status values:

- `idle` - Available for task assignment
- `busy` - Currently working
- `paused` - Temporarily disabled

## License

MIT
