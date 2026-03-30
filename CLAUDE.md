# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

TTAL is a CLI tool for managing projects, agents, workers, tasks, and daily focus. It uses TOML-based project storage and taskwarrior integration for task and today commands.

## Essential Commands

### Development Workflow
```bash
# Format, tidy, qlty, and build
make all

# Run all CI checks (qlty, test, build)
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

Projects are stored in a plain TOML file at `~/.config/ttal/projects.toml`. No database dependencies.

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
  ├── pr.go      - ttal pr create/modify/comment
  ├── worker.go  - ttal worker close/list
  ├── today.go   - ttal today list/completed/add/remove (daily focus)
  ├── task.go    - ttal task get/find (taskwarrior queries)
  ├── tag.go     - ttal tag (create + push git tags via daemon)
  └── go.go      - ttal go (pipeline stage engine)

internal/
  ├── agentfs/      - Filesystem-based agent discovery (CLAUDE.md frontmatter)
  ├── project/      - Project store (TOML) and resolution logic
  ├── promptrender/ - Unified prompt template renderer ($ cmd syntax)
  ├── watcher/      - JSONL file watcher (CC → Telegram via daemon)
  ├── daemon/       - Long-running daemon (socket, Telegram, delivery, launchd)
  ├── forgejo/      - Forgejo SDK client and repo helpers
  ├── pr/           - PR operations (create, modify, merge, comment)
  ├── worker/       - Worker lifecycle (hook, spawn, close)
  ├── gitutil/      - Git/worktree utilities (dump state, cleanup)
  ├── tmux/         - tmux session management and send-keys delivery
  ├── today/        - Today focus list (lipgloss tables, scheduled date mgmt)
  └── taskwarrior/  - Shared taskwarrior helpers (export, find, prompt, UDAs)
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
| `ttal go <uuid>` | Pipeline advance via CLI | `handlePipelineAdvance` → `advanceToStage` |
| `ttal tag <version>` | git tag + push via daemon | `handleGitTag` |
| Cleanup watcher (fsnotify) | Close worker + mark done | `startCleanupWatcher` → `worker.Close` → `MarkDone` |
| CC SessionStart hook | Session context injection | `ttal context` (installed via `ttal sync`) |

Socket protocol uses `SendRequest{From, To, Message}` — direction is inferred from which fields
are set. Taskwarrior hooks use `--to` (daemon socket → agent's tmux session).

The watcher (`internal/watcher/`) uses fsnotify to tail active CC session JSONL files. It maps
encoded project directory names back to registered agent paths, reads new bytes from tracked
offsets, and sends assistant text blocks to Telegram via the daemon's send callback. Agents write
normal text — the watcher handles routing to Telegram automatically.

The reviewer is advisory only — posts `VERDICT: LGTM` or `VERDICT: NEEDS_WORK` but never merges.
Even with LGTM, the coder triages remaining non-blocking issues before running `ttal go <uuid>`.
When `ttal go <uuid>` is run with `+lgtm` set, the daemon merges the PR (squash) and drops a
cleanup request file to `~/.ttal/cleanup/`. The daemon picks it up via fsnotify and handles the
full lifecycle: close session, remove worktree, mark task done.
`ttal doctor --fix` installs taskwarrior hooks (`on-add-ttal`, `on-modify-ttal`) and flicknote hooks.
`ttal sync` installs the CC SessionStart hook (`ttal context`) into `~/.claude/settings.json` — not `ttal doctor --fix`.

### Modify Command Syntax

The `modify` command supports field updates:

**Field Updates**: `field:value`
- Project fields: `alias`, `name`, `path`

```bash
ttal project modify clawd name:'New Name' path:/new/path
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
- Builds binaries for Linux/macOS (amd64, arm64)
- Creates Forgejo/GitHub release with binaries

### Git Hooks (qlty)

This repo uses [qlty](https://github.com/qltysh/qlty) for git hooks and unified linting. Install once:

```bash
qlty githooks install
# or: make install-hooks
```

The hooks run:
- **Pre-commit:** `qlty fmt` — auto-formats staged Go files (gofmt + goimports)
- **Pre-push:** `qlty check` — runs golangci-lint (16 linters via `.golangci.yml`) + trufflehog (secret scanning)
- **CI:** `qlty check --all` — full scan including osv-scanner (dependency vulns) + zizmor (GitHub Actions security) + trufflehog (secret scanning)

Workers in git worktrees may need to run `qlty githooks install` in their worktree directory.

**Important:** If a commit fails due to pre-commit hook, fix the issue and commit again. Do NOT use `--no-verify` to skip hooks.

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

### Package Documentation

Every `internal/` package has a `doc.go` with a plane annotation:

```go
// Package <name> <description>.
//
// Plane: manager|worker|shared
package <name>
```

- **manager** — daemon, long-running agent infrastructure
- **worker** — spawned worker/reviewer sessions
- **shared** — used by both planes or CLI commands

When creating new packages, add a `doc.go` with the appropriate plane tag.

Find all plane assignments: `grep -r "^// Plane:" internal/*/doc.go`

## Common Pitfalls

1. **Pushing directly to `main`** — branch protection requires a PR with passing CI. Always create a branch, push, and open a PR.

2. **Bypassing `internal/taskwarrior` with raw `exec.Command("task", ...)`**
   - Symptom: Ignores team TASKRC, no timeout, no `rc.verbose:nothing`
   - Fix: Always use the `internal/taskwarrior` package. If a helper doesn't exist (e.g. `StartTask`), add it there first — don't inline raw exec calls in `cmd/` or other packages.

3. **Unescaped HTML in Telegram messages** — When constructing HTML-formatted Telegram messages (`ParseModeHTML`), all dynamic/user-controlled strings must be wrapped with `html.EscapeString()` before embedding. Characters like `<`, `>`, `&` in task descriptions or user input will cause Telegram's HTML parser to reject the message. Escape at the caller side (where the HTML template is constructed), not in the transport layer.

4. **Hyphens in taskwarrior tags** — Tags must be alphanumeric + underscores only. Hyphens are parsed as argument separators: `+my-tag` becomes `+my` (add "my") and `-tag` (remove "tag"). Always use underscores: `+my_tag`.

## Agent Loop Design Principles

1. **Boundaries are structural.** Enforce limits in code, not prompts. Prompt rules are suggestions models can ignore — `maxSteps`, retry caps, and degenerate-loop detection are walls they cannot. Every prompt-level rule that matters (e.g. "one command per turn") needs a corresponding runtime guard.

2. **Boundary contact produces actionable feedback.** When an agent hits a limit, don't silently absorb it (`step--` forever) or crash with a bare error. Inject a clear, actionable message the model can act on — "You wrote multiple commands. Run one at a time." or "Summarize what you've found." The boundary is the constraint; the feedback is the recovery path.

## Secrets (.env)

All secrets live in `~/.config/ttal/.env` — bot tokens, API tokens, credentials.
They are injected into worker and agent sessions at spawn time.
Secrets are protected by CC native sandbox `denyRead` and `permissions.deny` entries
(written to `~/.claude/settings.json` by `ttal sync`).

```
# API tokens
GITHUB_TOKEN=ghp_...
FORGEJO_TOKEN=abc123...

# Bot tokens — convention: {UPPER_AGENT}_BOT_TOKEN
KESTREL_BOT_TOKEN=7123456:AAF...
ATHENA_BOT_TOKEN=7234567:AAG...
```

Generate a template: `ttal doctor --fix`

## Config Directory (`~/.config/ttal/`)

```
~/.config/ttal/
  ├── .env                    - Secrets (bot tokens, API keys)
  ├── config.toml             - Global ttal configuration
  ├── projects.toml           - Active/archived project registry
  ├── sandbox.toml            - Sandbox path config (feeds ttal sync → ~/.claude/settings.json)
  ├── roles.toml              - Agent role prompt templates (instructional text, no skills)
  ├── prompts.toml            - Prompt templates for agent operations
  └── license                 - License key
```

## Templates & Skills (SSOT)

The repo contains the **single source of truth** for agent definitions, skills, and subagents. `ttal sync` deploys subagents and rules to `~/.claude/agents/`, etc. Skills are stored in flicknote — use `ttal skill import <folder>` to upload them. **Edit here, not in `~/.claude/`** — runtime copies are overwritten by `ttal sync`.

### Source Directories

```
agents/                - Subagent definitions (→ ~/.claude/agents/)
  ├── plan-code-reviewer.md
  ├── pr-code-reviewer.md
  └── ...

templates/
  ttal/                - Agent identity files (frontmatter + CLAUDE.md)
    ├── CLAUDE.user.md - Global prompt (→ ~/.claude/CLAUDE.md via sync)
    ├── yuki.md        - Each agent's full identity, role, decision rules
    ├── kestrel.md
    └── ...

skills/                - Skill directories (each has SKILL.md)
  ├── sp-planning/SKILL.md        - Full planning process (explore → design → write → validate)
  ├── sp-debugging/SKILL.md       - Bug diagnosis + fix plans
  ├── sp-brainstorming/SKILL.md   - Brainstorming framework
  └── ...

commands/              - Static command .md files (flat)
  ├── tell-me-more.md  - Elaborate on a concept
  └── ...
```

### What Goes Where

| Type | Location | Format | How to deploy |
|------|----------|--------|---------------|
| Global prompt | `templates/ttal/CLAUDE.user.md` | Single `.md` file | `ttal sync` → `~/.claude/CLAUDE.md` |
| Skills (methodology) | `skills/` | Directory with `SKILL.md` | `ttal skill import skills --apply` |
| Subagents | `agents/` | Flat `.md` file | `ttal sync` → `~/.claude/agents/{name}.md` |
| Agent identities | `templates/ttal/` | Flat `.md` file | `ttal sync` → `~/.claude/agents/{name}.md` |
| Config TOMLs | `templates/ttal/` | `.toml` files | `ttal sync` → `~/.config/ttal/` |

**Global prompt:** `CLAUDE.user.md` is the SSOT for `~/.claude/CLAUDE.md`. All agents see this file as their global instructions. Edit `templates/ttal/CLAUDE.user.md`, then run `ttal sync` to deploy. Configured via `global_prompt_path` in `config.toml`'s `[sync]` section.

**Skills:** Skills live in flicknote and are accessed at runtime via `ttal skill get`. Import from source with `ttal skill import skills --apply`. Dynamic commands also use flicknote — trigger via Telegram sends `run ttal skill get <name>` to the agent.

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
