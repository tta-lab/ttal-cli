# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

TTAL is a CLI tool for managing projects, agents, workers, tasks, and daily focus with tag-based filtering and routing. It uses a schema-first database approach with type-safe queries, plus taskwarrior integration for task and today commands.

## Essential Commands

### Development Workflow
```bash
# After modifying ent schemas (CRITICAL - must run after any schema change)
make generate

# Format, generate, vet, and build
make all

# Run all CI checks (format, generate, vet, lint, test, build)
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

### Database Management
```bash
# Remove database (destructive!)
make clean-db

# Full reset (binary + database)
make reset
```

## Architecture

### Database Layer - ent (Schema-First ORM)

The project uses [ent](https://entgo.io/) as a type-safe, schema-first ORM (similar to Drizzle for Go). This is the **single source of truth** for the database.

**Schema Location**: `ent/schema/`
- `project.go` - Project entity with fields and edges
- `agent.go` - Agent entity with fields and edges
- `tag.go` - Tag entity (shared by projects and agents via M2M)

**Critical Workflow**:
1. Modify schema files in `ent/schema/`
2. Run `make generate` to regenerate type-safe query code
3. ent auto-generates ~3000 lines of code in `ent/` directory
4. **Never manually edit** generated files (they're regenerated on every `make generate`)

**Benefits over raw SQL**:
- Type-safe queries with compile-time checking
- Automatic migrations (no manual SQL)
- M2M relations handled automatically
- No manual row scanning
- Reduced codebase by 73% (see ENT_REFACTOR.md)

**Example Query Pattern**:
```go
// Fetch projects with eager-loaded tags
projects, err := database.Project.Query().
    WithTags().
    Where(project.ArchivedAtIsNil()).
    All(ctx)
```

### Project Structure

```
cmd/             - CLI commands (cobra)
  ├── root.go    - Root command and database initialization
  ├── project.go - Project CRUD commands
  ├── agent.go   - Agent CRUD commands
  ├── memory.go  - Memory capture command
  ├── daemon.go  - ttal daemon run/install/uninstall/status
  ├── send.go    - ttal send --to (messaging)
  ├── pr.go      - ttal pr create/modify/merge/comment
  ├── worker.go  - ttal worker spawn/close/list
  ├── today.go   - ttal today list/completed/add/remove (daily focus)
  └── task.go    - ttal task get/find (taskwarrior queries)

ent/             - ent ORM (mostly auto-generated)
  └── schema/    - Schema definitions (source of truth)

internal/
  ├── db/        - Database connection wrapper
  ├── memory/    - Memory capture logic (git commit extraction)
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
| on-add hook (task created) | Background `claude -p` enrichment | `HookOnAdd` → `HookEnrich` |
| on-modify hook (task started) | Background `ttal worker spawn` | `handleOnStart` → `HookSpawnWorker` |
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

### Tag-Based Routing

Projects and agents use tags for automatic matching:
- Tags use taskwarrior-like syntax: `+tag` (add), `-tag` (remove)
- Agents can see projects that share **at least one tag**
- Tags are stored in separate M2M tables managed by ent
- Tag names are case-insensitive (auto-lowercased)

**Example**:
```bash
# Agent with +secretary +core tags
ttal agent add yuki +secretary +core

# Project with +core +infrastructure tags
ttal project add clawd +core +infrastructure

# yuki can see clawd (both have +core)
ttal agent info yuki  # Shows clawd in matching projects
```

### Modify Command Syntax

The `modify` command supports both field updates and tag operations:

**Field Updates**: `field:value`
- Agent fields: `path`
- Project fields: `name`, `description`, `path`, `repo`, `repo-type`, `owner`

**Tag Operations**: `+tag` (add), `-tag` (remove)

**Important**: Always use `--` separator before modifications to prevent `-tag` being interpreted as a flag.

```bash
# Correct
ttal project modify clawd -- +backend -legacy name:'New Name'

# Wrong (will fail)
ttal project modify clawd +backend -legacy  # -legacy treated as flag
```

## Commit Convention

Use this format for commits:
```
ttal: [category] description

Examples:
ttal: impl - add memory capture
ttal: refactor - optimize tag queries
ttal: fix - handle nil archived_at
```

## CI/CD

### Workflows (.forgejo/workflows/)

**pr.yaml** - Runs on PRs:
- Checks formatting, vet, linting
- Verifies ent generated code is up-to-date (checks for uncommitted changes after `make generate`)
- Runs tests and builds binary

**ci.yaml** - Runs on push to main:
- Full build and lint checks
- Keeps main branch healthy

**release.yaml** - Triggers on version tags (e.g., `v1.0.0`):
- Builds binaries for Linux/macOS/Windows (amd64, arm64)
- Creates Forgejo/GitHub release with binaries

### Pre-commit Hooks

Install hooks to run checks automatically:
```bash
make install-hooks
```

The hook runs: fmt → generate → vet → test

## Testing

Tests use ent's test utilities:
- `ent/enttest` - In-memory SQLite test database
- `internal/db/testutil.go` - Shared test helpers

**Run tests**:
```bash
make test          # All tests
go test ./cmd/     # Command tests only
go test -v ./...   # Verbose output
```

## Common Pitfalls

1. **Forgot to run `make generate`** after schema changes
   - Symptom: Type errors, missing methods, CI failure
   - Fix: Always run `make generate` after editing `ent/schema/`

2. **Using `-tag` without `--` separator**
   - Symptom: "unknown flag: -tag"
   - Fix: Use `ttal modify alias -- -tag`

3. **Editing generated ent files**
   - Symptom: Changes disappear after `make generate`
   - Fix: Only edit `ent/schema/`, never generated files

4. **Manual database migrations**
   - Not needed! ent handles migrations automatically via `Schema.Create()`

5. **NEVER delete the database file directly**
   - ⚠️ **CRITICAL**: Never run `rm ~/.ttal/ttal.db` - this deletes ALL user data
   - Tests use in-memory databases (`internal/db/testutil.go`) - they never touch `~/.ttal/ttal.db`
   - To clean up test data: Use CLI commands (`ttal agent delete`, `ttal project archive`)
   - Only use `make clean-db` when explicitly instructed by the user
   - The database contains real user data and has no backup mechanism

## Database Location

Default: `~/.ttal/ttal.db` (SQLite with WAL mode)

Override with global flag:
```bash
ttal --db=/custom/path/ttal.db project list
```

## Additional Documentation

- `README.md` - User-facing documentation and usage
- `ENT_REFACTOR.md` - Detailed comparison of ent vs raw SQL
- `docs/DATABASE.md` - Database schema details
- `CI_CD_SETUP.md` - CI/CD pipeline documentation
- `TESTING.md` - Testing guidelines
- `docs/plans/2026-02-17-daemon-design.md` - Daemon design doc (see implementation note at top for API changes)
- `docs/TELEGRAM_LIB_DECISION.md` - Why we chose go-telegram/bot
