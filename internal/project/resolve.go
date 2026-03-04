package project

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/tta-lab/ttal-cli/ent"
	"github.com/tta-lab/ttal-cli/ent/project"
	"github.com/tta-lab/ttal-cli/internal/config"
	"github.com/tta-lab/ttal-cli/internal/db"
)

// ResolveProjectPath looks up a project path by matching the taskwarrior
// project field against the ttal project DB alias.
// Returns empty string if no match found (caller should notify lifecycle agent).
//
// Resolution order:
//  1. If projectName matches an alias (with hierarchical fallback: "ttal.pr" → "ttal") → use that project's path
//  2. If projectName contains exactly one alias ("ttal-cli" contains "ttal") → use that project's path
//  3. If no match but only ONE project exists in DB → use it (single-project shortcut)
//  4. Otherwise → return empty (no match)
func ResolveProjectPath(projectName string) string {
	dbPath := config.ResolveDBPath()
	if _, err := os.Stat(dbPath); err != nil {
		return ""
	}

	database, err := db.New(dbPath)
	if err != nil {
		return ""
	}
	defer database.Close()

	ctx := context.Background()

	// Try hierarchical candidates: "ttal.pr" → try "ttal.pr", then "ttal"
	if projectName != "" {
		candidates := []string{projectName}
		parts := strings.Split(projectName, ".")
		for i := len(parts) - 1; i >= 1; i-- {
			candidates = append(candidates, strings.Join(parts[:i], "."))
		}

		for _, candidate := range candidates {
			proj, err := database.Project.Query().
				Where(project.Alias(candidate), project.ArchivedAtIsNil()).
				Only(ctx)
			if err != nil {
				if !ent.IsNotFound(err) {
					fmt.Fprintf(os.Stderr, "resolve: unexpected error querying alias %q: %v\n", candidate, err)
				}
				continue
			}
			if proj.Path != "" {
				return proj.Path
			}
		}
	}

	// Fetch all active projects once for contains fallback and single-project shortcut.
	allProjects, err := database.Project.Query().
		Where(project.ArchivedAtIsNil()).
		All(ctx)
	if err != nil {
		return ""
	}

	// Contains fallback: "ttal-cli" matches alias "ttal" because "ttal-cli" contains "ttal"
	if projectName != "" {
		if path := matchByContains(projectName, allProjects); path != "" {
			return path
		}
	}

	// Single-project shortcut: if only one active project in DB, always use it.
	if len(allProjects) == 1 && allProjects[0].Path != "" {
		return allProjects[0].Path
	}

	return ""
}

// ValidateProjectAlias checks that a project alias exists in the ttal DB (exact match).
// Returns a user-friendly error listing available projects if not found.
func ValidateProjectAlias(alias string) error {
	dbPath := config.ResolveDBPath()
	if _, err := os.Stat(dbPath); err != nil {
		return fmt.Errorf("project database not found — run `ttal project add` first")
	}

	database, err := db.New(dbPath)
	if err != nil {
		return fmt.Errorf("failed to open project database: %w", err)
	}
	defer database.Close()

	ctx := context.Background()
	_, err = database.Project.Query().
		Where(project.Alias(alias), project.ArchivedAtIsNil()).
		Only(ctx)
	if err != nil {
		if ent.IsNotFound(err) {
			return formatProjectNotFoundError(alias, database.Client, ctx)
		}
		return fmt.Errorf("project lookup failed: %w", err)
	}
	return nil
}

func formatProjectNotFoundError(alias string, database *ent.Client, ctx context.Context) error {
	projects, _ := database.Project.Query().
		Where(project.ArchivedAtIsNil()).
		All(ctx)

	aliases := make([]string, 0, len(projects))
	for _, p := range projects {
		aliases = append(aliases, p.Alias)
	}

	msg := fmt.Sprintf("project %q not found\n\nAvailable projects:\n  %s\n\nUse `ttal project list` to see all projects.",
		alias, strings.Join(aliases, ", "))
	return fmt.Errorf("%s", msg)
}

// matchByContains finds a project whose alias is contained within the input name.
// Returns the project path only if exactly one project matches (no ambiguity).
// Empty aliases are skipped to avoid false matches (strings.Contains(s, "") is always true).
func matchByContains(name string, projects []*ent.Project) string {
	var matches []*ent.Project
	lower := strings.ToLower(name)
	for _, p := range projects {
		if p.Path != "" && p.Alias != "" && strings.Contains(lower, strings.ToLower(p.Alias)) {
			matches = append(matches, p)
		}
	}
	if len(matches) == 1 {
		return matches[0].Path
	}
	return ""
}
