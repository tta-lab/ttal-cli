package claudeconfig

import (
	"encoding/json"
	"fmt"
	"os"
)

// UpsertTrust reads (or creates) a .claude.json file and ensures all
// given project paths have trust entries. Returns count of added entries.
func UpsertTrust(claudeJSONPath string, projectPaths []string) (int, error) {
	if len(projectPaths) == 0 {
		return 0, nil
	}

	raw, err := readOrCreateClaudeJSON(claudeJSONPath)
	if err != nil {
		return 0, err
	}

	projects, err := extractProjects(raw, claudeJSONPath)
	if err != nil {
		return 0, err
	}

	added := 0
	for _, path := range projectPaths {
		if isTrusted(projects[path]) {
			continue
		}
		projects[path] = NewProjectTrustEntry()
		added++
	}

	if added == 0 {
		return 0, nil
	}

	out, merr := json.MarshalIndent(raw, "", "  ")
	if merr != nil {
		return 0, fmt.Errorf("marshal %s: %w", claudeJSONPath, merr)
	}
	if werr := os.WriteFile(claudeJSONPath, out, 0o644); werr != nil {
		return 0, fmt.Errorf("write %s: %w", claudeJSONPath, werr)
	}
	return added, nil
}

// readOrCreateClaudeJSON reads ~/.claude.json into a raw map, or returns a
// fresh map with hasCompletedOnboarding if the file does not exist yet.
func readOrCreateClaudeJSON(claudeJSONPath string) (map[string]any, error) {
	data, err := os.ReadFile(claudeJSONPath)
	if os.IsNotExist(err) {
		return map[string]any{"hasCompletedOnboarding": true}, nil
	}
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", claudeJSONPath, err)
	}
	var raw map[string]any
	if uerr := json.Unmarshal(data, &raw); uerr != nil {
		return nil, fmt.Errorf("parse %s: %w", claudeJSONPath, uerr)
	}
	if raw == nil {
		return map[string]any{"hasCompletedOnboarding": true}, nil
	}
	return raw, nil
}

// extractProjects returns the projects map from raw, creating it if absent.
// Returns an error if "projects" exists but is not a JSON object.
func extractProjects(raw map[string]any, claudeJSONPath string) (map[string]any, error) {
	v, exists := raw["projects"]
	if !exists {
		projects := make(map[string]any)
		raw["projects"] = projects
		return projects, nil
	}
	m, ok := v.(map[string]any)
	if !ok {
		return nil, fmt.Errorf("parse %s: \"projects\" field has unexpected type %T", claudeJSONPath, v)
	}
	return m, nil
}

// isTrusted reports whether a project entry already has hasTrustDialogAccepted set to true.
func isTrusted(entry any) bool {
	m, ok := entry.(map[string]any)
	return ok && m["hasTrustDialogAccepted"] == true
}

// NewProjectTrustEntry returns a trust entry map for a CC project.
func NewProjectTrustEntry() map[string]any {
	return map[string]any{
		"hasTrustDialogAccepted":        true,
		"hasCompletedProjectOnboarding": true,
		"allowedTools":                  []any{},
	}
}
