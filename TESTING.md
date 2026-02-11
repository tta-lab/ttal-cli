# Testing Guide

## Overview

This project uses Go's built-in testing framework with in-memory SQLite databases for isolated, fast tests.

## Test Structure

### Test Database Setup

All tests use **in-memory SQLite databases** via `/internal/db/testutil.go`:

```go
// Creates isolated in-memory database for each test
database = db.NewTestDB(t)
```

**Benefits:**
- ✅ **No cleanup required** - Database is destroyed when test ends
- ✅ **Fast** - In-memory is much faster than disk-based SQLite
- ✅ **Isolated** - Each test gets its own database
- ✅ **No interference** - Doesn't touch user's actual `~/.ttal/ttal.db`

### Test Files

#### `cmd/agent_test.go`
Tests for agent management commands:
- `TestParseModifyArgs` - Tests parsing of modify command arguments (tags + fields)
- `TestAgentModifyPath` - Tests modifying agent path
- `TestAgentModifyTags` - Tests adding/removing agent tags
- `TestAgentModifyCombined` - Tests modifying path and tags together
- `TestAgentQueryWithTags` - Tests querying agents by tags

#### `cmd/project_test.go`
Tests for project management commands:
- `TestProjectModifyName` - Tests modifying project name
- `TestProjectModifyDescription` - Tests adding/modifying description
- `TestProjectModifyPath` - Tests modifying project path
- `TestProjectModifyRepo` - Tests modifying repo information
- `TestProjectModifyRepoType` - Tests setting repo type (forgejo/github/codeberg)
- `TestProjectModifyTags` - Tests adding/removing project tags
- `TestProjectModifyMultipleFields` - Tests modifying multiple fields at once
- `TestProjectModifyCombined` - Tests modifying fields and tags together
- `TestProjectArchive` - Tests archiving/unarchiving projects
- `TestProjectQueryWithTags` - Tests querying projects by tags

## Running Tests

```bash
# Run all tests
make test

# Run tests in a specific package
go test -v ./cmd

# Run a specific test
go test -v ./cmd -run TestAgentModifyPath

# Run tests with coverage
go test -cover ./...

# Run tests with coverage report
go test -coverprofile=coverage.out ./...
go tool cover -html=coverage.out
```

## Test Coverage

Current test coverage focuses on:
- ✅ Modify command argument parsing (`+tag`, `-tag`, `field:value`)
- ✅ Agent field modifications (path)
- ✅ Project field modifications (name, description, path, repo, repo-type, owner)
- ✅ Tag operations (add, remove, query)
- ✅ Combined operations (fields + tags in one command)
- ✅ Archive/unarchive functionality
- ✅ Tag-based filtering

## CI/CD Integration

Tests run automatically in CI via:
- `.forgejo/workflows/pr.yaml` - Runs on all pull requests
- `.forgejo/workflows/ci.yaml` - Runs on push to main
- `make ci` - Local CI check (includes fmt, generate, vet, lint, test, build)

## Writing New Tests

### Template for Command Tests

```go
func TestYourFeature(t *testing.T) {
    setupAgentTest(t)  // or setupProjectTest(t)
    ctx := context.Background()

    // Create test data
    // ...

    // Execute operation
    // ...

    // Verify results
    // ...
}
```

### Best Practices

1. **Use in-memory database** - Call `setupAgentTest(t)` or `setupProjectTest(t)`
2. **Name tests clearly** - Use descriptive names like `TestAgentModifyPath`
3. **Test one thing** - Each test should verify one specific behavior
4. **Clean variable names** - Use `ag` for agent entities, `proj` for project entities (avoid shadowing package imports)
5. **Add subtests** - Use `t.Run()` for testing multiple scenarios
6. **Check errors** - Always verify that operations complete successfully

## Database Isolation

Each test gets a completely isolated in-memory database:

```
Test 1: memory://db1 ➜ [Agent A, Tag X]
Test 2: memory://db2 ➜ [Project B, Tag Y]
Test 3: memory://db3 ➜ [Agent C, Project D]
```

No cross-contamination, no cleanup needed!
