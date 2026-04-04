package project

import (
	"fmt"
	"path/filepath"
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
	return resolveProjectPathWithStore(projectName, NewStore(config.ResolveProjectsPath()))
}

// ResolveProjectPathForTeam is like ResolveProjectPath but reads from the specified
// team's projects.toml instead of the cached active team. Use this when the team
// is known at call time (e.g. from a CleanupRequest) to avoid sync.Once cache issues
// in the daemon where config.ResolveProjectsPath() is cached at startup.
// When team is empty, falls back to ResolveProjectPath behavior.
func ResolveProjectPathForTeam(projectName, team string) string {
	if team == "" {
		return ResolveProjectPath(projectName)
	}
	return resolveProjectPathWithStore(projectName, NewStore(config.ResolveProjectsPathForTeam(team)))
}

// ResolveProjectAlias returns the project alias for a given filesystem path.
// Returns the alias if the path is inside (or equal to) a registered project path,
// or if the path is a ttal worktree whose directory name contains a valid alias.
// Otherwise returns "" — callers fall back to GITHUB_TOKEN.
func ResolveProjectAlias(workDir string) string {
	return resolveProjectAliasWithStore(workDir, NewStore(config.ResolveProjectsPath()), "")
}

// resolveProjectAliasWithStore resolves alias using a provided store.
// worktreesRoot overrides config.WorktreesRoot() — pass "" to use the default.
func resolveProjectAliasWithStore(workDir string, store *Store, worktreesRoot string) string {
	projects, err := store.List(false)
	if err != nil {
		return ""
	}

	cleanWork := filepath.Clean(workDir)

	// 1. Path prefix match against registered project paths
	for _, p := range projects {
		cleanProj := filepath.Clean(p.Path)
		if cleanWork == cleanProj || strings.HasPrefix(cleanWork, cleanProj+string(filepath.Separator)) {
			return p.Alias
		}
	}

	// 2. Worktree path: <worktreesRoot>/<uuid8>-<alias>/ → extract alias
	if worktreesRoot == "" {
		worktreesRoot = config.WorktreesRoot()
	}
	cleanRoot := filepath.Clean(worktreesRoot)
	if strings.HasPrefix(cleanWork, cleanRoot+string(filepath.Separator)) {
		rel := strings.TrimPrefix(cleanWork, cleanRoot)
		rel = strings.TrimPrefix(rel, string(filepath.Separator))
		parts := strings.SplitN(rel, string(filepath.Separator), 2)
		if len(parts) >= 1 {
			dir := parts[0]
			// Format: <uuid8>-<alias> where uuid8 is exactly 8 hex chars
			if len(dir) > 9 && dir[8] == '-' {
				alias := dir[9:]
				// Validate alias exists in store
				if proj, err := store.Get(alias); err == nil && proj != nil && alias != "" {
					return alias
				}
			}
		}
	}

	// 3. Otherwise: return "" (caller falls back to GITHUB_TOKEN)
	return ""
}

// ResolveProjectPathOrError resolves a project path from a taskwarrior project field.
// Returns a user-friendly error if the project alias is not registered.
func ResolveProjectPathOrError(projectName string) (string, error) {
	return resolveProjectPathOrErrorWithStore(projectName, NewStore(config.ResolveProjectsPath()))
}

// resolveProjectPathOrErrorWithStore is the store-injectable implementation of
// ResolveProjectPathOrError, used directly by tests to avoid real config reads.
func resolveProjectPathOrErrorWithStore(projectName string, store *Store) (string, error) {
	path := resolveProjectPathWithStore(projectName, store)
	if path != "" {
		return path, nil
	}
	if projectName == "" {
		return "", fmt.Errorf("task has no project field set")
	}
	// Extract base alias for the error message ("ttal.pr" → "ttal")
	baseAlias := projectName
	if i := strings.Index(projectName, "."); i > 0 {
		baseAlias = projectName[:i]
	}
	return "", formatProjectNotFoundError(baseAlias, store)
}

// resolveProjectPathWithStore performs the resolution logic using a provided store.
// Shared by ResolveProjectPath and ResolveProjectPathOrError to avoid double store opens.
func resolveProjectPathWithStore(projectName string, store *Store) string {
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
				continue // I/O error on this candidate — try next
			}
			if proj != nil && proj.Path != "" {
				return proj.Path
			}
		}
	}

	allProjects, err := store.List(false)
	if err != nil {
		return ""
	}

	if projectName != "" {
		if path := matchByContains(projectName, allProjects); path != "" {
			return path
		}
	}

	if len(allProjects) == 1 && allProjects[0].Path != "" {
		return allProjects[0].Path
	}

	return ""
}

// GetProjectPath looks up a project by exact alias and returns its path.
// Returns a user-friendly error listing available projects if not found.
// If the alias contains "." and a hierarchical parent exists, suggests it.
func GetProjectPath(alias string) (string, error) {
	store := NewStore(config.ResolveProjectsPath())
	proj, err := store.Get(alias)
	if err != nil {
		return "", fmt.Errorf("project lookup failed: %w", err)
	}
	if proj != nil {
		if proj.Path == "" {
			return "", fmt.Errorf("project %q exists but has no path configured", alias)
		}
		return proj.Path, nil
	}

	// Not found — check for hierarchical "did you mean?" suggestion
	if i := strings.Index(alias, "."); i > 0 {
		base := alias[:i]
		baseProj, baseErr := store.Get(base)
		if baseErr == nil && baseProj != nil {
			return "", fmt.Errorf("project %q not found — did you mean %q?", alias, base)
		}
	}

	return "", formatProjectNotFoundError(alias, store)
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

// ResolveProject looks up a project by taskwarrior project name using hierarchical resolution.
// Returns the full Project struct (not just path). Returns nil if no match found.
//
// Resolution order (same as ResolveProjectPath):
//  1. Hierarchical candidates: "ttal.pr" → try "ttal.pr", then "ttal"
//  2. Contains fallback: "ttal-cli" matches alias "ttal"
//  3. Single-project shortcut
func ResolveProject(projectName string) *Project {
	return resolveProjectWithStore(projectName, NewStore(config.ResolveProjectsPath()))
}

// ResolveProjectForTeam is like ResolveProject but reads from the specified team's store.
func ResolveProjectForTeam(projectName, team string) *Project {
	if team == "" {
		return ResolveProject(projectName)
	}
	return resolveProjectWithStore(projectName, NewStore(config.ResolveProjectsPathForTeam(team)))
}

func resolveProjectWithStore(projectName string, store *Store) *Project {
	if projectName != "" {
		// Hierarchical candidates: "ttal.pr" → try "ttal.pr", then "ttal"
		candidates := []string{projectName}
		parts := strings.Split(projectName, ".")
		for i := len(parts) - 1; i >= 1; i-- {
			candidates = append(candidates, strings.Join(parts[:i], "."))
		}

		for _, candidate := range candidates {
			proj, err := store.Get(candidate)
			if err != nil {
				continue
			}
			if proj != nil && proj.Path != "" {
				return proj
			}
		}
	}

	// Contains fallback
	allProjects, err := store.List(false)
	if err != nil {
		return nil
	}

	if projectName != "" {
		if p := matchProjectByContains(projectName, allProjects); p != nil {
			return p
		}
	}

	// Single-project shortcut
	if len(allProjects) == 1 && allProjects[0].Path != "" {
		return &allProjects[0]
	}

	return nil
}

// matchProjectByContains finds a project whose alias is contained within the input name.
// Returns the full Project struct only if exactly one project matches (no ambiguity).
// Empty aliases are skipped to avoid false matches (strings.Contains(s, "") is always true).
func matchProjectByContains(name string, projects []Project) *Project {
	var matches []Project
	lower := strings.ToLower(name)
	for _, p := range projects {
		if p.Path != "" && p.Alias != "" && strings.Contains(lower, strings.ToLower(p.Alias)) {
			matches = append(matches, p)
		}
	}
	if len(matches) == 1 {
		return &matches[0]
	}
	return nil
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
