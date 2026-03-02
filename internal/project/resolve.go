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
