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
  - Run qlty check (golangci-lint with 16 linters + trufflehog secret scanning)

**Purpose:** Ensure all PRs meet quality standards before merge

### 2. CI Workflow (`.github/workflows/ci.yaml`)

**Trigger:** Push to `main` branch

**Jobs:**
- **build**
  - Generate JSON Schema
  - Run tests
  - Build binary

- **lint**
  - Run qlty check (golangci-lint + trufflehog + osv-scanner + zizmor)

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

**Enabled Linters (16):**
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

### Git Hooks (qlty)

**Installed via:** `qlty githooks install` or `make install-hooks`

**Pre-commit hook:**
- `qlty fmt` — auto-formats staged Go files (gofmt + goimports)

**Pre-push hook:**
- `qlty check` — runs golangci-lint (16 linters via `.golangci.yml`) + trufflehog (secret scanning)

Tests are CI-only and do not run in git hooks.

## Makefile Enhancements

### New Targets

```bash
# CI - runs all checks (same as CI workflow)
make ci

# Check if working directory is clean (for CI)
make check-clean

# Install qlty git hooks
make install-hooks

# Run qlty check (lint + security scan)
make qlty
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
make qlty          # Run qlty check (lint + security scan)
make all           # Format, tidy, schema, qlty, and build
make ci            # Run all CI checks (qlty, schema, test, build)
make check-clean   # Check if working directory is clean
make install-hooks # Install qlty git hooks
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
- [ ] Qlty check passes (`make qlty`)
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
# 1. Install qlty hooks (one-time)
qlty githooks install

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
