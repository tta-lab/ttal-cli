# TTAL CLI - Project Reference Manager

A command-line tool for managing projects, agents, and automated memory capture with tag-based filtering and agent routing.

## Features

- **Project Management**: Add, list, archive, and tag projects
- **Agent Management**: Configure agents with tags, status tracking, and heartbeat periods
- **Tag-Based Filtering**: Taskwarrior-like syntax for tag management (`+tag` to add, `-tag` to remove)
- **Agent Routing**: Automatic project matching based on shared tags
- **Memory Capture**: Extract git commits and generate agent-filtered memory logs

## Installation

```bash
# Build from source
make build

# Or use go directly
go build -o ttal

# Install to GOPATH/bin
make install
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

## Tag System

Tags use taskwarrior-like syntax:

- `+tag` - Add a tag
- `-tag` - Remove a tag (use `--` before tags when removing: `ttal project modify alias -- -tag`)
- `+tag1 +tag2` - Add multiple tags
- `-old +new` - Mix add and remove operations (use `--`: `ttal project modify alias -- -old +new`)

### Agent-Project Matching

Agents can see projects that share at least one tag:

```bash
# Agent with tags: +secretary +core
ttal agent add yuki +secretary +core

# Project with tags: +core +infrastructure
ttal project add clawd +core +infrastructure

# yuki can work on clawd (both have +core tag)
ttal agent info yuki  # Shows clawd in matching projects
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
