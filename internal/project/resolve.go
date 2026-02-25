package project

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"codeberg.org/clawteam/ttal-cli/ent"
	"codeberg.org/clawteam/ttal-cli/ent/project"
	"codeberg.org/clawteam/ttal-cli/internal/config"
	"codeberg.org/clawteam/ttal-cli/internal/db"
)

// ResolveProjectPath looks up a project path by matching the taskwarrior
// project field against the ttal project DB alias.
// Returns empty string if no match found (caller should notify lifecycle agent).
//
// Resolution order:
//  1. If projectName matches an alias (with hierarchical fallback: "ttal.pr" → "ttal") → use that project's path
//  2. If no match but only ONE project exists in DB → use it (single-project shortcut)
//  3. Otherwise → return empty (no match)
func ResolveProjectPath(projectName string) string {
	dbPath := filepath.Join(config.ResolveDataDir(), "ttal.db")
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

	// Single-project shortcut: if only one active project in DB, always use it.
	projects, err := database.Project.Query().
		Where(project.ArchivedAtIsNil()).
		All(ctx)
	if err != nil {
		return ""
	}
	if len(projects) == 1 && projects[0].Path != "" {
		return projects[0].Path
	}

	return ""
}
