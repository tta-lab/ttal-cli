# CI/CD Setup Documentation

## Overview

This document describes the CI/CD workflows and development tooling implemented for ttal-cli, based on patterns from cloudnative-supabase.

## Workflows

### 1. PR Workflow (`.github/workflows/pr.yaml`)

**Trigger:** Pull requests to `main` branch

**Jobs:**
- **build**
  - Setup Go (version from go.mod)
  - Download dependencies
  - Generate JSON Schema
  - Check for uncommitted changes (fails if generated schema is stale)
  - Run tests
  - Build binary

- **lint**
  - Format check (fails if code not formatted)
  - Run go vet
  - Run golangci-lint with 17 linters

**Purpose:** Ensure all PRs meet quality standards before merge

### 2. CI Workflow (`.github/workflows/ci.yaml`)

**Trigger:** Push to `main` branch

**Jobs:**
- **build**
  - Generate JSON Schema
  - Run tests
  - Build binary

- **lint**
  - Format code
  - Run go vet
  - Run golangci-lint

**Purpose:** Keep main branch healthy and catch issues immediately after merge

### 3. Release Workflow (`.github/workflows/release.yaml`)

**Trigger:** Version tags (e.g., `v1.0.0`)

**Steps:**
1. Run tests
2. Build binaries for supported platforms:
   - `ttal-linux-amd64`
   - `ttal-linux-arm64`
   - `ttal-darwin-amd64`
   - `ttal-darwin-arm64`
3. Generate SHA256 checksums
4. Create GitHub/Forgejo release with binaries

**Usage:**
```bash
git tag v1.0.0
git push origin v1.0.0
# Release is automatically created
```

## Code Quality Tools

### golangci-lint Configuration (`.golangci.yml`)

**Enabled Linters (17):**
- `copyloopvar` - Detect loop variable capture issues
- `dupl` - Find duplicated code
- `errcheck` - Check for unchecked errors
- `goconst` - Find repeated strings that should be constants
- `gocyclo` - Detect complex functions (threshold: 15)
- `govet` - Standard Go static analysis
- `ineffassign` - Detect ineffectual assignments
- `lll` - Line length check (max: 120)
- `misspell` - Spell checking
- `nakedret` - Naked returns in long functions
- `prealloc` - Suggest slice preallocation
- `revive` - General-purpose linter
- `staticcheck` - Advanced static analysis
- `unconvert` - Unnecessary type conversions
- `unparam` - Unused function parameters
- `unused` - Unused code detection

**Formatters:**
- `gofmt` - Standard Go formatting
- `goimports` - Import organization

**Exclusions:**
- Line length checks relaxed for generated code

### Pre-commit Hook (`scripts/pre-commit.sh`)

**Installed via:** `make install-hooks`

**Checks performed:**
1. Stash unstaged changes (to only check what's being committed)
2. Run `make fmt` (auto-apply formatting)
3. Run `make vet` (static analysis)
4. Run `make test` (all tests)
5. Restore stashed changes

**Behavior:**
- Auto-formats code and stages changes
- Fails commit if vet or tests fail
- Keeps working directory clean

## Makefile Enhancements

### New Targets

```bash
# CI - runs all checks (same as CI workflow)
make ci

# Check if working directory is clean (for CI)
make check-clean

# Install pre-commit hook
make install-hooks
```

### Complete Target List

```bash
make build         # Build the ttal binary
make install       # Install ttal to GOPATH/bin
make run           # Run ttal (usage: make run ARGS='project list')
make clean         # Remove built binaries
make test          # Run tests
make schema        # Regenerate JSON Schema from config structs
make fmt           # Format code with gofmt
make vet           # Run go vet
make lint          # Run golangci-lint (if installed)
make all           # Format, schema, vet, and build
make ci            # Run all CI checks
make check-clean   # Check if working directory is clean
make install-hooks # Install git pre-commit hook
```

## PR Template (`.forgejo/pull_request_template.md`)

Standardized template for all pull requests:

**Sections:**
- Summary - Brief description
- Changes - List of key changes
- Testing - How changes were tested
- Checklist - Pre-merge requirements

**Required checks before merge:**
- [ ] Code formatted (`make fmt`)
- [ ] Tests pass (`make test`)
- [ ] Vet passes (`make vet`)
- [ ] JSON Schema updated (`make schema`)
- [ ] Conventional commit format used
- [ ] No sensitive data committed

## .gitignore Updates

Added exclusions for:
- Release binaries (`ttal-*`)
- Build directory (`bin/`)
- CI artifacts (`checksums.txt`, `dist/`)

## Development Workflow

### Standard Flow

```bash
# 1. Install pre-commit hook (one-time)
make install-hooks

# 2. Make changes to code

# 3. Run checks locally (optional, pre-commit will do this)
make ci

# 4. Commit (pre-commit hook runs automatically)
git commit -m "feat(scope): description"

# 5. Push and create PR
git push origin feature-branch
```

### PR Requirements

1. All CI checks must pass
2. Code must be formatted
3. Generated JSON Schema must be up-to-date
4. Tests must pass
5. golangci-lint must not report errors
6. Conventional commit format required

### Conventional Commit Format

```
<type>(<scope>): <description>

Types: feat, fix, chore, refactor, docs, test, ci
Examples:
  feat(cli): add project list command
  fix(db): resolve connection leak
  chore(deps): update golangci-lint to v1.60
  ci(lint): add golangci-lint configuration
```


## References

- Based on: `/Users/neil/Code/guion/cloudnative-supabase`
- Forgejo Actions docs: https://forgejo.org/docs/latest/user/actions/
- golangci-lint docs: https://golangci-lint.run/
