# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

TTAL is a CLI tool for managing projects, agents, workers, tasks, and daily focus with tag-based filtering and routing. It uses TOML-based project storage and taskwarrior integration for task and today commands.

## Essential Commands

### Development Workflow
```bash
# Format, tidy, schema, vet, and build
make all

# Run all CI checks (format, schema, vet, lint, test, build)
make ci

# Run tests
make test

# Build binary
make build

# Install to GOPATH/bin
make install
```

### Running the CLI
```bash
# Build and run with arguments
make run ARGS='project list'

# Or use the built binary directly
./ttal project list
```

## Releasing

Tag a version to trigger the release workflow:

```bash
git tag v0.1.0
git push origin v0.1.0
```

GoReleaser builds binaries, creates a GitHub release, and pushes the
Homebrew formula to `tta-lab/homebrew-ttal`. Users upgrade via `brew upgrade ttal`.

Requires `HOMEBREW_TAP_TOKEN` secret in GitHub repo settings (a PAT with repo scope on the tap repo).

## Architecture

### Project Storage — TOML File

Projects are stored in a plain TOML file at `~/.config/ttal/projects.toml` (or `~/.config/ttal/{team}-projects.toml` for non-default teams). No database dependencies.

**Store Location**: `internal/project/store.go`

**TOML Format**:
```toml
# Active projects are top-level sections
[ttal]
name = "TTAL Core"
path = "/Users/neil/Code/guion-opensource/ttal-cli"

[clawd]
name = "Clawd Workspace"
path = "/Users/neil/clawd"

# Archived projects go under [archived]
[archived.old-project]
name = "Legacy Thing"
path = "/old/path"
```

The table key IS the alias. Active projects are top-level `[alias]`, archived under `[archived.alias]`.

**Project Resolution** (`internal/project/resolve.go`):
```go
// Resolution order for taskwarrior project matching:
// 1. Exact alias match (with "." hierarchical fallback)
// 2. Contains fallback ("ttal-cli" matches alias "ttal")
// 3. Single-project shortcut (if only one project exists)
path := project.ResolveProjectPath("ttal.pr")
```

**Agent metadata** is stored in CLAUDE.md frontmatter files (see `internal/agentfs/`), not in the project store.

### Project Structure

```
cmd/             - CLI commands (cobra)
  ├── root.go    - Root command and .env loading
  ├── project.go - Project CRUD commands (TOML-backed)
  ├── agent.go   - Agent CRUD commands
  ├── daemon.go  - ttal daemon run/install/uninstall/status
  ├── send.go    - ttal send --to (messaging)
  ├── pr.go      - ttal pr create/modify/merge/comment
  ├── worker.go  - ttal worker spawn/close/list
  ├── today.go   - ttal today list/completed/add/remove (daily focus)
  ├── task.go    - ttal task get/find (taskwarrior queries)
  └── task_route.go - ttal task design/research/test/execute (routing + spawn)

internal/
  ├── agentfs/   - Filesystem-based agent discovery (CLAUDE.md frontmatter)
  ├── project/   - Project store (TOML) and resolution logic
  ├── watcher/   - JSONL file watcher (CC → Telegram via daemon)
  ├── daemon/    - Long-running daemon (socket, Telegram, delivery, launchd)
  ├── forgejo/   - Forgejo SDK client and repo helpers
  ├── pr/        - PR operations (create, modify, merge, comment)
  ├── worker/    - Worker lifecycle (hook, spawn, close)
  ├── gitutil/   - Git/worktree utilities (dump state, cleanup)
  ├── tmux/      - tmux session management and send-keys delivery
  ├── today/     - Today focus list (lipgloss tables, scheduled date mgmt)
  └── taskwarrior/ - Shared taskwarrior helpers (export, find, prompt, UDAs)
```

### Daemon Architecture

The daemon (`internal/daemon/`) is a communication hub managed by launchd. It handles all
inter-agent and human-agent messaging. **Do not add fallback logic** — each path is explicit:

| Path | Channel | Handler |
|---|---|---|
| JSONL watcher (fsnotify) | Telegram (outbound) | `watcher.Watcher` |
| `ttal send --to kestrel` | tmux send-keys | `handleTo` |
| `ttal send --to kestrel` (with TTAL_AGENT_NAME) | tmux send-keys + attribution | `handleAgentToAgent` |
| on-add hook (task created) | Inline enrichment (project_path, branch) | `HookOnAdd` → `enrichInline` |
| `ttal task execute <uuid>` | Worker spawn via CLI | `spawnWorkerForTask` → `worker.Spawn` |
| `ttal task design <uuid>` | Daemon socket → agent tmux | `resolveAgentByRole("designer")` → `routeTaskToAgent` |
| `ttal task research <uuid>` | Daemon socket → agent tmux | `resolveAgentByRole("researcher")` → `routeTaskToAgent` |
| `ttal task route <uuid> --to X` | Daemon socket → agent tmux | `agentfs.Get` → `routeTaskToAgent` |
| Cleanup watcher (fsnotify) | Close worker + mark done | `startCleanupWatcher` → `worker.Close` → `MarkDone` |

Socket protocol uses `SendRequest{From, To, Message}` — direction is inferred from which fields
are set. Taskwarrior hooks use `--to` (daemon socket → agent's tmux session).

The watcher (`internal/watcher/`) uses fsnotify to tail active CC session JSONL files. It maps
encoded project directory names back to registered agent paths, reads new bytes from tracked
offsets, and sends assistant text blocks to Telegram via the daemon's send callback. Agents write
normal text — the watcher handles routing to Telegram automatically.

The reviewer is advisory only — posts `VERDICT: LGTM` or `VERDICT: NEEDS_WORK` but never merges.
Even with LGTM, the coder triages remaining non-blocking issues before merging. The coder
runs `ttal pr merge` after triage, which drops a cleanup request file to `~/.ttal/cleanup/`.
The daemon picks it up via fsnotify and handles the full lifecycle: close session, remove
worktree, mark task done.
`ttal worker install` installs both `on-add-ttal` and `on-modify-ttal` taskwarrior hooks.

### Modify Command Syntax

The `modify` command supports field updates:

**Field Updates**: `field:value`
- Project fields: `alias`, `name`, `path`

```bash
ttal project modify clawd name:'New Name' path:/new/path
```

## Commit Convention

Use this format for commits:
```
ttal: [category] description

Examples:
ttal: impl - add worker spawn
ttal: refactor - optimize tag queries
ttal: fix - handle nil archived_at
```

## CI/CD

### Workflows (.github/workflows/)

**pr.yaml** - Runs on PRs:
- Checks formatting, vet, linting
- Verifies generated schema is up-to-date
- Runs tests and builds binary

**ci.yaml** - Runs on push to main:
- Full build and lint checks
- Keeps main branch healthy

**release.yaml** - Triggers on version tags (e.g., `v1.0.0`):
- Builds binaries for Linux/macOS/Windows (amd64, arm64)
- Creates Forgejo/GitHub release with binaries

### Pre-commit Hooks (lefthook)

This repo uses [lefthook](https://github.com/evilmartians/lefthook) for pre-commit hooks. Install once in the main repo:
```bash
lefthook install
```

The pre-commit hook runs **fmt, vet, lint** in parallel. Tests are CI-only.

Workers in git worktrees inherit hooks from the main repo automatically.

**Important:** If a commit fails due to pre-commit hook, fix the issue (usually `make fmt`) and commit again. Do NOT use `--no-verify` to skip hooks.

## Testing

Tests use temp-file TOML stores for project operations:
- `internal/project/store_test.go` - Store unit tests
- `cmd/project_test.go` - Project command integration tests

**Run tests**:
```bash
make test          # All tests
go test ./cmd/     # Command tests only
go test -v ./...   # Verbose output
```

## Common Pitfalls

1. **Bypassing `internal/taskwarrior` with raw `exec.Command("task", ...)`**
   - Symptom: Ignores team TASKRC, no timeout, no `rc.verbose:nothing`
   - Fix: Always use the `internal/taskwarrior` package. If a helper doesn't exist (e.g. `StartTask`), add it there first — don't inline raw exec calls in `cmd/` or other packages.

## Secrets (.env)

All secrets live in `~/.config/ttal/.env` — bot tokens, API tokens, credentials.
They are injected into worker and agent sessions at spawn time.

```
# API tokens
GITHUB_TOKEN=ghp_...
FORGEJO_TOKEN=abc123...

# Bot tokens — convention: {UPPER_AGENT}_BOT_TOKEN
KESTREL_BOT_TOKEN=7123456:AAF...
ATHENA_BOT_TOKEN=7234567:AAG...
```

Generate a template: `ttal doctor --fix`

## Project Storage Location

Default: `~/.config/ttal/projects.toml`

Per-team: `~/.config/ttal/{team}-projects.toml`

## Additional Documentation

- `README.md` - User-facing documentation and usage
- `CI_CD_SETUP.md` - CI/CD pipeline documentation
- `TESTING.md` - Testing guidelines
- `docs/plans/2026-02-17-daemon-design.md` - Daemon design doc (see implementation note at top for API changes)
- `docs/TELEGRAM_LIB_DECISION.md` - Why we chose go-telegram/bot
- `docs/ECOSYSTEM.md` - GuionAI ecosystem overview (FlickNote + TTAL)
- `docs/AIOPS.md` - AIOps system overview and stack
- `docs/guides/` - Architecture guide series (philosophy and "why" behind TTAL)
- `docs/posts/` - Blog posts (unpublished drafts for dev.to/HN)
