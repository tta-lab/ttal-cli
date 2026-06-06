package project

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"charm.land/lipgloss/v2"
)

// Project represents a project entry from the `project` CLI.
type Project struct {
	Alias          string `json:"alias,omitempty"`
	Name           string `json:"name,omitempty"`
	Path           string `json:"path,omitempty"`
	Remote         string `json:"remote,omitempty"`
	GitHubTokenEnv string `json:"github_token_env,omitempty"`
	K8sApp         string `json:"k8s_app,omitempty"`
	K8sNamespace   string `json:"k8s_namespace,omitempty"`
}

// Get looks up a project by exact alias and returns its full info.
// Returns nil if not found or on error.
func Get(alias string) (*Project, error) {
	return getProjectByAlias(alias)
}

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
	return resolveProjectPathInner(projectName)
}

// ResolveProjectAlias returns the project alias for a given filesystem path.
// Returns the alias if the path is inside (or equal to) a registered project path,
// or if the path is a ttal worktree whose directory name contains a valid alias.
// Otherwise returns "" — callers fall back to GITHUB_TOKEN.
func ResolveProjectAlias(workDir string) string {
	return resolveProjectAliasInner(workDir)
}

// ResolveProjectPathOrError resolves a project path from a taskwarrior project field.
// Returns a user-friendly error if the project alias is not registered.
func ResolveProjectPathOrError(projectName string) (string, error) {
	path := resolveProjectPathInner(projectName)
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
	return "", formatProjectNotFoundError(baseAlias)
}

// GetProjectPath looks up a project by exact alias and returns its path.
// Returns a user-friendly error listing available projects if not found.
// If the alias contains "." and a hierarchical parent exists, suggests it.
func GetProjectPath(alias string) (string, error) {
	proj, err := getProjectByAlias(alias)
	if err != nil {
		// If hierarchical fallback succeeded: did-you-mean suggestion
		if fallbackProj, fbErr := getProjectByAliasHierarchical(alias); fbErr == nil && fallbackProj != nil {
			return "", fmt.Errorf("project %q not found — did you mean %q?", alias, fallbackProj.Alias)
		}
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
		if baseProj, err := getProjectByAlias(base); err == nil && baseProj != nil {
			return "", fmt.Errorf("project %q not found — did you mean %q?", alias, base)
		}
	}

	return "", formatProjectNotFoundError(alias)
}

// ValidateProjectAlias checks that a project alias exists (exact match, active only).
// Returns a user-friendly error listing available projects if not found.
func ValidateProjectAlias(alias string) error {
	_, err := getProjectByAlias(alias)
	if err != nil {
		return formatProjectNotFoundError(alias)
	}
	return nil
}

// ResolveProject looks up a project by taskwarrior project name using hierarchical resolution.
// Returns the full Project struct (not just path). Returns nil if no match found.
//
// Resolution order (same as ResolveProjectPath):
//  1. Hierarchical candidates: "ttal.pr" → try "ttal.pr", then "ttal"
//  2. Contains fallback: "ttal-cli" matches alias "ttal"
//  3. Single-project shortcut
func ResolveProject(projectName string) *Project {
	return resolveProjectInner(projectName)
}

// List loads all projects from the `project` CLI.
func List() ([]Project, error) {
	return loadProjects()
}

// ResolveGitHubToken returns the GitHub token for a project.
func ResolveGitHubToken(projectAlias string) string {
	if projectAlias == "" {
		return os.Getenv("GITHUB_TOKEN")
	}
	proj := resolveProjectInner(projectAlias)
	if proj == nil || proj.GitHubTokenEnv == "" {
		return os.Getenv("GITHUB_TOKEN")
	}
	if token := os.Getenv(proj.GitHubTokenEnv); token != "" {
		return token
	}
	return os.Getenv("GITHUB_TOKEN")
}

// --- shell-out helpers ---

func runProjectJSON(args ...string) ([]byte, error) {
	cmd := exec.Command("project", args...)
	cmd.Stderr = nil // suppress stderr
	return cmd.Output()
}

func loadProjects() ([]Project, error) {
	out, err := runProjectJSON("list", "--json")
	if err != nil {
		return nil, fmt.Errorf("project list failed: %w", err)
	}
	var projs []Project
	if err := json.Unmarshal(out, &projs); err != nil {
		return nil, fmt.Errorf("parsing project list: %w", err)
	}
	return projs, nil
}

func getProjectByAlias(alias string) (*Project, error) {
	out, err := runProjectJSON("resolve", alias)
	if err != nil {
		return nil, fmt.Errorf("project %q not found", alias)
	}
	var proj Project
	if err := json.Unmarshal(out, &proj); err != nil {
		return nil, fmt.Errorf("parsing project resolve: %w", err)
	}
	if proj.Alias == "" {
		return nil, fmt.Errorf("project %q not found", alias)
	}
	return &proj, nil
}

func getProjectByPath(targetPath string) (*Project, error) {
	out, err := runProjectJSON("resolve", targetPath)
	if err != nil {
		return nil, fmt.Errorf("project for path %q not found", targetPath)
	}
	var proj Project
	if err := json.Unmarshal(out, &proj); err != nil {
		return nil, fmt.Errorf("parsing project resolve: %w", err)
	}
	if proj.Alias == "" {
		return nil, fmt.Errorf("project for path %q not found", targetPath)
	}
	return &proj, nil
}

// getProjectByAliasHierarchical tries hierarchical fallback: "fb.ap" → "fb"
func getProjectByAliasHierarchical(alias string) (*Project, error) {
	parts := strings.Split(alias, ".")
	if len(parts) <= 1 {
		return nil, fmt.Errorf("no parent for %q", alias)
	}
	parent := strings.Join(parts[:len(parts)-1], ".")
	return getProjectByAlias(parent)
}

// --- ttal-specific resolution logic (kept in Go) ---

func resolveProjectPathInner(projectName string) string {
	if projectName == "" {
		return ""
	}

	// Hierarchical candidates: "ttal.pr" → try "ttal.pr", then "ttal"
	candidates := []string{projectName}
	parts := strings.Split(projectName, ".")
	for i := len(parts) - 1; i >= 1; i-- {
		candidates = append(candidates, strings.Join(parts[:i], "."))
	}

	for _, candidate := range candidates {
		proj, err := getProjectByAlias(candidate)
		if err == nil && proj != nil && proj.Path != "" {
			return proj.Path
		}
	}

	// Contains fallback
	allProjects, err := loadProjects()
	if err != nil {
		return ""
	}

	if projectName != "" {
		if path := matchProjectPathByContains(projectName, allProjects); path != "" {
			return path
		}
	}

	// Single-project shortcut
	if len(allProjects) == 1 && allProjects[0].Path != "" {
		return allProjects[0].Path
	}

	return ""
}

func resolveProjectInner(projectName string) *Project {
	if projectName == "" {
		return nil
	}

	candidates := []string{projectName}
	parts := strings.Split(projectName, ".")
	for i := len(parts) - 1; i >= 1; i-- {
		candidates = append(candidates, strings.Join(parts[:i], "."))
	}

	for _, candidate := range candidates {
		proj, err := getProjectByAlias(candidate)
		if err == nil && proj != nil && proj.Path != "" {
			return proj
		}
	}

	// Contains fallback
	allProjects, err := loadProjects()
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

func resolveProjectAliasInner(workDir string) string {
	cleanWork := filepath.Clean(workDir)

	// Try path-based lookup via `project resolve <path>`
	if proj, err := getProjectByPath(cleanWork); err == nil && proj != nil {
		return proj.Alias
	}

	// Try path prefix match — load all and check
	projs, err := loadProjects()
	if err == nil {
		for _, p := range projs {
			if p.Path != "" {
				cleanProj := filepath.Clean(p.Path)
				if cleanWork == cleanProj || strings.HasPrefix(cleanWork, cleanProj+string(filepath.Separator)) {
					return p.Alias
				}
			}
		}
	}

	return ""
}

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

func matchProjectPathByContains(name string, projects []Project) string {
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

func formatProjectNotFoundError(alias string) error {
	projs, _ := loadProjects()
	aliases := make([]string, 0, len(projs))
	for _, p := range projs {
		aliases = append(aliases, p.Alias)
	}
	msg := fmt.Sprintf("project %q not found\n\nAvailable projects:\n  %s\n\nUse `ttal project list` to see all projects.",
		alias, strings.Join(aliases, ", "))
	return fmt.Errorf("%s", msg)
}

// Ensure lipgloss is used (for statusline table formatting)
var _ = lipgloss.NewStyle
