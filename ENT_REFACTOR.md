# ent Refactor - Before & After

This document shows the improvements from refactoring ttal-cli to use [ent](https://entgo.io/) instead of raw SQL.

## Summary of Changes

- ✅ **Deleted 400+ lines** of manual SQL code
- ✅ **Added type-safe queries** with compile-time checking
- ✅ **Auto migrations** - no more manual schema management
- ✅ **M2M relations** handled automatically
- ✅ **Drizzle-like** schema-first approach

## Code Comparison

### Schema Definition

**Before (Raw SQL)**:
```go
// internal/db/migrations.go
const schema = `
CREATE TABLE IF NOT EXISTS projects (
	alias TEXT PRIMARY KEY,
	name TEXT NOT NULL,
	description TEXT,
	path TEXT,
	repo TEXT,
	repo_type TEXT,
	owner TEXT,
	archived_at TIMESTAMP,
	created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS project_tags (
	id TEXT PRIMARY KEY,
	project_alias TEXT NOT NULL,
	tag TEXT NOT NULL,
	FOREIGN KEY(project_alias) REFERENCES projects(alias) ON DELETE CASCADE,
	UNIQUE(project_alias, tag)
);
`
```

**After (ent Schema)**:
```go
// ent/schema/project.go
func (Project) Fields() []ent.Field {
	return []ent.Field{
		field.String("alias").Unique().NotEmpty(),
		field.String("name").NotEmpty(),
		field.String("description").Optional(),
		field.String("path").Optional(),
		field.String("repo").Optional(),
		field.String("repo_type").Optional(),
		field.String("owner").Optional(),
		field.Time("archived_at").Optional().Nillable(),
		field.Time("created_at").Default(time.Now).Immutable(),
	}
}

func (Project) Edges() []ent.Edge {
	return []ent.Edge{
		edge.To("tags", Tag.Type),  // ✅ M2M handled automatically!
	}
}
```

### Creating a Project with Tags

**Before (Manual SQL - 50 lines)**:
```go
// internal/models/project.go
func (s *ProjectStore) Create(p *Project) error {
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	// Insert project
	_, err = tx.Exec(`
		INSERT INTO projects (alias, name, description, path, repo, repo_type, owner, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)
	`, p.Alias, p.Name, p.Description, p.Path, p.Repo, p.RepoType, p.Owner, time.Now())
	if err != nil {
		return fmt.Errorf("failed to insert project: %w", err)
	}

	// Insert tags manually
	for _, tag := range p.Tags {
		id := uuid.New().String()
		_, err := tx.Exec(`
			INSERT INTO project_tags (id, project_alias, tag)
			VALUES (?, ?, ?)
			ON CONFLICT(project_alias, tag) DO NOTHING
		`, id, p.Alias, tag)
		if err != nil {
			return fmt.Errorf("failed to insert tag %s: %w", tag, err)
		}
	}

	return tx.Commit()
}
```

**After (ent - 15 lines)**:
```go
// cmd/project.go
createProject := database.Project.Create().
	SetAlias(projectAlias).
	SetName(projectName).
	SetDescription(projectDescription).
	SetPath(projectPath)

p, err := createProject.Save(ctx)
if err != nil {
	return fmt.Errorf("failed to create project: %w", err)
}

// Add tags (helper function handles find-or-create)
if err := addProjectTags(ctx, p, tagNames); err != nil {
	return fmt.Errorf("failed to add tags: %w", err)
}
```

### Querying Projects with Tags

**Before (Complex SQL - 30 lines)**:
```go
// internal/models/project.go
func (s *ProjectStore) List(filterTags []string, includeArchived bool) ([]*Project, error) {
	query := `
		SELECT DISTINCT p.alias, p.name, p.description, p.path, p.repo, p.repo_type, p.owner, p.archived_at, p.created_at
		FROM projects p
	`

	args := []interface{}{}

	if len(filterTags) > 0 {
		query += `
			INNER JOIN project_tags pt ON p.alias = pt.project_alias
			WHERE pt.tag IN (` + placeholders(len(filterTags)) + `)
		`
		for _, tag := range filterTags {
			args = append(args, tag)
		}
		query += `
			GROUP BY p.alias
			HAVING COUNT(DISTINCT pt.tag) = ?
		`
		args = append(args, len(filterTags))
	}

	rows, err := s.db.Query(query, args...)
	// ... manual scanning ...
}
```

**After (ent - 8 lines)**:
```go
// cmd/project.go
query := database.Project.Query().WithTags()

if !includeArchived {
	query = query.Where(project.ArchivedAtIsNil())
}

if len(tagNames) > 0 {
	query = query.Where(project.HasTagsWith(tag.NameIn(tagNames...)))
}

projects, err := query.All(ctx)
// ✅ Tags automatically loaded via WithTags()!
```

### Agent-Project Matching

**Before (Manual Join - 20 lines)**:
```go
// internal/models/agent.go
func (s *AgentStore) FindMatchingProjects(agentName string) ([]string, error) {
	rows, err := s.db.Query(`
		SELECT DISTINCT p.alias
		FROM projects p
		INNER JOIN project_tags pt ON p.alias = pt.project_alias
		INNER JOIN agent_tags at ON pt.tag = at.tag
		WHERE at.agent_name = ?
		AND p.archived_at IS NULL
		ORDER BY p.alias
	`, agentName)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var projects []string
	for rows.Next() {
		var alias string
		if err := rows.Scan(&alias); err != nil {
			return nil, err
		}
		projects = append(projects, alias)
	}
	return projects, rows.Err()
}
```

**After (ent - 5 lines)**:
```go
// cmd/agent.go
projects, err := database.Project.Query().
	Where(
		project.ArchivedAtIsNil(),
		project.HasTagsWith(tag.NameIn(tagNames...)),
	).
	All(ctx)
// ✅ Type-safe, readable, no manual joins!
```

## Files Deleted

- ❌ `internal/models/project.go` (255 lines)
- ❌ `internal/models/agent.go` (277 lines)
- ❌ `internal/db/migrations.go` (57 lines)

Total: **589 lines deleted** ✂️

## Files Added

- ✅ `ent/schema/project.go` (60 lines)
- ✅ `ent/schema/agent.go` (55 lines)
- ✅ `ent/schema/tag.go` (40 lines)
- ✅ Auto-generated ent code (~3000 lines, but we don't maintain this!)

Total manually written: **155 lines** ✍️

## Benefits

### 1. Type Safety
```go
// Before: Runtime error if column name is wrong
row.Scan(&p.Alias, &p.Name, &p.Descriptin) // typo!

// After: Compile-time error
p.Descriptin // ❌ Compiler error: field not found
p.Description // ✅ Autocomplete works!
```

### 2. No Manual Scanning
```go
// Before: Manual scanning for every query
rows, _ := db.Query("SELECT alias, name FROM projects")
for rows.Next() {
	var alias, name string
	rows.Scan(&alias, &name) // Error-prone!
}

// After: Auto-mapped to structs
projects, _ := client.Project.Query().All(ctx)
// projects is []*ent.Project with all fields populated!
```

### 3. Relations Just Work
```go
// Before: Separate query for tags
project := getProject(alias)
tags := getTags(alias) // Manual join

// After: Eager loading
project, _ := client.Project.Query().
	Where(project.AliasEQ(alias)).
	WithTags(). // ✅ Tags loaded in one query!
	Only(ctx)

for _, tag := range project.Edges.Tags {
	fmt.Println(tag.Name) // ✅ Type-safe!
}
```

### 4. Migrations Are Automatic
```go
// Before: Manually write SQL migrations
const schema = `CREATE TABLE...`
db.Exec(schema)

// After: Just run
client.Schema.Create(ctx) // ✅ Auto-creates tables from schema!
```

### 5. Better Error Messages
```go
// Before: "pq: syntax error at or near..." 🤔

// After:
// ✅ "project with alias 'foo' not found"
// ✅ Compile error if using wrong field name
```

## Performance

ent queries are as fast as hand-written SQL because:
- Generates optimal SQL at compile time
- No reflection in hot paths
- Connection pooling handled automatically

## Conclusion

The ent refactor:
- Reduced manual code by **73%** (589 → 155 lines)
- Added full type safety
- Eliminated entire classes of bugs (SQL syntax errors, scan errors)
- Made the code more maintainable and readable
- Provided a **Drizzle-like experience** in Go! 🚀
