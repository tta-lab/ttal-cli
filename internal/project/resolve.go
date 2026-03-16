package project

import (
	"fmt"
	"strings"

	"github.com/tta-lab/ttal-cli/internal/config"
)

// ResolveProjectPath looks up a project path by matching the taskwarrior
// project field against the ttal project alias.
// Returns empty string if no match found (caller should notify lifecycle agent).
//
// Resolution order:
//  1. If projectName matches an alias (with hierarchical fallback: "ttal.pr" → "ttal") → use that project's path
//  2. If projectName contains exactly one alias ("ttal-cli" contains "ttal") → use that project's path
//  3. If no match but only ONE project exists → use it (single-project shortcut)
//  4. Otherwise → return empty (no match)
func ResolveProjectPath(projectName string) string {
	store := NewStore(config.ResolveProjectsPath())

	// Try hierarchical candidates: "ttal.pr" → try "ttal.pr", then "ttal"
	if projectName != "" {
		candidates := []string{projectName}
		parts := strings.Split(projectName, ".")
		for i := len(parts) - 1; i >= 1; i-- {
			candidates = append(candidates, strings.Join(parts[:i], "."))
		}

		for _, candidate := range candidates {
			proj, err := store.Get(candidate)
			if err != nil {
				return ""
			}
			if proj != nil && proj.Path != "" {
				return proj.Path
			}
		}
	}

	// Fetch all active projects for contains fallback and single-project shortcut.
	allProjects, err := store.List(false)
	if err != nil {
		return ""
	}

	// Contains fallback: "ttal-cli" matches alias "ttal" because "ttal-cli" contains "ttal"
	if projectName != "" {
		if path := matchByContains(projectName, allProjects); path != "" {
			return path
		}
	}

	// Single-project shortcut: if only one active project, always use it.
	if len(allProjects) == 1 && allProjects[0].Path != "" {
		return allProjects[0].Path
	}

	return ""
}

// ResolveProjectPathOrError resolves a project path from a taskwarrior project field.
// Returns a user-friendly error if the project alias is not registered.
func ResolveProjectPathOrError(projectName string) (string, error) {
	path := ResolveProjectPath(projectName)
	if path != "" {
		return path, nil
	}
	if projectName == "" {
		return "", fmt.Errorf("task has no project field set")
	}
	// Extract base alias for the error message
	baseAlias := projectName
	if i := strings.Index(projectName, "."); i > 0 {
		baseAlias = projectName[:i]
	}
	return "", formatProjectNotFoundError(baseAlias, NewStore(config.ResolveProjectsPath()))
}

// ValidateProjectAlias checks that a project alias exists (exact match, active only).
// Returns a user-friendly error listing available projects if not found.
func ValidateProjectAlias(alias string) error {
	store := NewStore(config.ResolveProjectsPath())

	proj, err := store.Get(alias)
	if err != nil {
		return fmt.Errorf("project lookup failed: %w", err)
	}
	if proj != nil {
		return nil
	}

	return formatProjectNotFoundError(alias, store)
}

func formatProjectNotFoundError(alias string, store *Store) error {
	projects, _ := store.List(false)

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
func matchByContains(name string, projects []Project) string {
	var matches []Project
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
