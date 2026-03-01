# TTAL Database Guide

## Overview

TTAL uses SQLite with ent (Entity Framework for Go) for type-safe, schema-first database management. The database handles projects, agents, and their tag-based relationships.

## Database Location

**Default location:** `~/.ttal/ttal.db`

**Custom location:** Override with `--db` flag:
```bash
ttal --db /path/to/custom.db project list
```

## Database Files

When using WAL (Write-Ahead Logging) mode, you'll see:

- **`ttal.db`** - Main database file (persistent)
- **`ttal.db-shm`** - Shared memory file (temporary, created when active)
- **`ttal.db-wal`** - Write-Ahead Log file (temporary, checkpoint auto-flushes to main DB)

The `-shm` and `-wal` files are automatically managed by SQLite and may not always be present.

## Initialization Mechanism

### Automatic Initialization

TTAL automatically initializes the database on first use. **Zero configuration required.**

When you run any `ttal` command, the system:

1. **Creates the directory** if it doesn't exist (`~/.ttal/`)
2. **Opens SQLite connection** with optimized settings
3. **Runs auto-migrations** to create/update schema
4. **Closes connection** cleanly after command completes

### Implementation Details

From `internal/db/db.go`:

```go
func New(dbPath string) (*DB, error) {
    // 1. Ensure directory exists
    dir := filepath.Dir(dbPath)
    os.MkdirAll(dir, 0755)

    // 2. Open SQLite with recommended settings
    dsn := fmt.Sprintf("file:%s?cache=shared&_fk=1&_journal_mode=WAL&_busy_timeout=5000", dbPath)
    client, err := ent.Open("sqlite3", dsn)

    // 3. Run auto-migrations
    client.Schema.Create(context.Background())

    return &DB{client}, nil
}
```

### Connection Settings

The DSN (Data Source Name) includes these optimizations:

| Setting | Value | Purpose |
|---------|-------|---------|
| `cache=shared` | Enabled | Better concurrency for multiple connections |
| `_fk=1` | Enabled | Enforces foreign key constraints (critical for M2M relations) |
| `_journal_mode=WAL` | Enabled | Write-Ahead Logging for better performance + concurrent reads |
| `_busy_timeout=5000` | 5 seconds | Wait up to 5s on lock conflicts before failing |

**Why WAL mode?**
- ✅ **Concurrent reads** while writes are in progress
- ✅ **Better performance** - writes don't block readers
- ✅ **Atomic commits** - safer crash recovery
- ⚠️ **Trade-off**: Requires `-shm` and `-wal` files (auto-managed)

## Schema

The database uses three main tables with many-to-many tag relationships:

```sql
-- Core entities
CREATE TABLE projects (
    alias TEXT PRIMARY KEY,
    name TEXT NOT NULL,
    description TEXT,
    path TEXT,
    archived_at DATETIME,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE agents (
    name TEXT PRIMARY KEY,
    path TEXT,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE tags (
    id TEXT PRIMARY KEY,
    name TEXT NOT NULL UNIQUE
);

-- M2M junction tables (auto-created by ent)
CREATE TABLE project_tags (
    project_alias TEXT NOT NULL,
    tag_id TEXT NOT NULL,
    PRIMARY KEY (project_alias, tag_id),
    FOREIGN KEY (project_alias) REFERENCES projects(alias) ON DELETE CASCADE,
    FOREIGN KEY (tag_id) REFERENCES tags(id) ON DELETE CASCADE
);

CREATE TABLE agent_tags (
    agent_name TEXT NOT NULL,
    tag_id TEXT NOT NULL,
    PRIMARY KEY (agent_name, tag_id),
    FOREIGN KEY (agent_name) REFERENCES agents(name) ON DELETE CASCADE,
    FOREIGN KEY (tag_id) REFERENCES tags(id) ON DELETE CASCADE
);
```

### Key Constraints

- **Agent names**: Lowercase only (validated at schema level)
- **Tag names**: Lowercase only (validated at schema level)
- **Project alias**: Unique identifier (primary key)
- **Repo type**: Enum validation (forgejo, github, codeberg)
- **Tags**: Shared across projects and agents via M2M relations
- **Cascading deletes**: Removing a project/agent removes its tag associations

## Migration System

### Auto-Migration

Every command run triggers `client.Schema.Create()` which:

- ✅ Creates tables if they don't exist
- ✅ Adds missing columns
- ✅ Updates indexes and constraints
- ✅ Creates junction tables for M2M relations
- ⚠️ **Does NOT drop columns** (safe for existing data)
- ⚠️ **Does NOT modify existing column types** (manual migration needed)

### Schema Evolution

When you update `ent/schema/*.go` files:

1. Regenerate ent code: `go generate ./ent`
2. Run any ttal command to auto-migrate
3. ent handles the schema diff automatically

**Example workflow:**
```bash
# 1. Edit schema
vim ent/schema/project.go

# 2. Regenerate ent code
go generate ./ent

# 3. Auto-migration happens on next command
ttal project list  # Triggers migration
```

### Manual Migrations

For complex changes (column type changes, data migrations), see ent's migration guide:
https://entgo.io/docs/versioned-migrations

## Database Maintenance

### Clean Up Database

**Remove all data (fresh start):**
```bash
rm -f ~/.ttal/ttal.db ~/.ttal/ttal.db-shm ~/.ttal/ttal.db-wal
```

The database will be recreated on next command.

### Backup Database

**Manual backup:**
```bash
cp ~/.ttal/ttal.db ~/ttal-backup-$(date +%Y%m%d).db
```

**With WAL checkpoint (ensures all data is in main file):**
```bash
sqlite3 ~/.ttal/ttal.db "PRAGMA wal_checkpoint(TRUNCATE);"
cp ~/.ttal/ttal.db ~/ttal-backup-$(date +%Y%m%d).db
```

### Restore Database

```bash
cp ~/ttal-backup-20260211.db ~/.ttal/ttal.db
```

### Inspect Database

**Open SQLite shell:**
```bash
sqlite3 ~/.ttal/ttal.db
```

**Common commands:**
```sql
-- Show schema
.schema

-- List tables
.tables

-- Show all projects
SELECT * FROM projects;

-- Show tag relationships
SELECT p.alias, t.name
FROM projects p
JOIN project_tags pt ON p.alias = pt.project_alias
JOIN tags t ON pt.tag_id = t.id;

-- Exit
.quit
```

### Check Database Size

```bash
du -h ~/.ttal/ttal.db
```

### Vacuum Database (compact size)

```bash
sqlite3 ~/.ttal/ttal.db "VACUUM;"
```

## Troubleshooting

### "Database is locked" Error

**Cause:** Another process has a write lock on the database.

**Solutions:**
1. Wait for the other process to finish (busy_timeout=5000ms)
2. Check for stuck processes: `lsof ~/.ttal/ttal.db`
3. As last resort: `rm ~/.ttal/ttal.db-shm ~/.ttal/ttal.db-wal`

### "Foreign key constraint failed"

**Cause:** Trying to reference a non-existent entity (e.g., adding tag to non-existent project).

**Solution:** Ensure the referenced entity exists first.

### Migration Fails

**Symptoms:** `failed to run migrations: ...`

**Debugging:**
```bash
# Check schema
sqlite3 ~/.ttal/ttal.db ".schema"

# Check for corrupt database
sqlite3 ~/.ttal/ttal.db "PRAGMA integrity_check;"

# Last resort: rebuild
rm ~/.ttal/ttal.db
ttal project list  # Recreates from scratch
```

### Lowercase Validation Errors

**Symptoms:** `agent name must be lowercase` or `tag name must be lowercase`

**Solution:** The CLI auto-converts to lowercase, but if using the ent client directly, ensure lowercase values.

## Performance Tips

### Indexes

ent automatically creates indexes for:
- Primary keys (alias, name, id)
- Unique constraints (tag.name)
- Foreign keys (M2M junction tables)
- Custom indexes (e.g., projects.archived_at)

### Query Optimization

**Eager load relationships:**
```go
// Good: Single query with JOIN
projects, err := client.Project.Query().
    WithTags().  // Eager load tags
    All(ctx)

// Bad: N+1 queries
projects, err := client.Project.Query().All(ctx)
for _, p := range projects {
    tags, _ := p.QueryTags().All(ctx)  // Separate query per project!
}
```

**Use filtering at database level:**
```go
// Good: Filter in SQL
projects, err := client.Project.Query().
    Where(project.ArchivedAtIsNil()).
    All(ctx)

// Bad: Filter in Go
projects, err := client.Project.Query().All(ctx)
filtered := make([]*ent.Project, 0)
for _, p := range projects {
    if p.ArchivedAt == nil {
        filtered = append(filtered, p)
    }
}
```

## Development

### Schema Files (Manual)

These are the only files you should edit:

- `ent/schema/project.go` - Project entity definition
- `ent/schema/agent.go` - Agent entity definition
- `ent/schema/tag.go` - Tag entity definition
- `ent/generate.go` - Generation directive

### Generated Files (Auto)

**Never edit these** - they're regenerated by `go generate ./ent`:

- `ent/*.go` - CRUD builders and client
- `ent/project/*.go` - Project types and predicates
- `ent/agent/*.go` - Agent types and predicates
- `ent/tag/*.go` - Tag types and predicates
- `ent/migrate/schema.go` - Migration definitions

### Regenerating Code

After editing schema files:

```bash
go generate ./ent
```

This runs: `entgo.io/ent/cmd/ent generate ./schema`

## References

- **ent Documentation**: https://entgo.io/docs/getting-started
- **SQLite WAL Mode**: https://www.sqlite.org/wal.html
- **SQLite Pragmas**: https://www.sqlite.org/pragma.html
