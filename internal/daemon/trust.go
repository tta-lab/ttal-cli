package daemon

import (
	"encoding/json"
	"fmt"
	"os"
)

// upsertClaudeJSONTrust adds hasTrustDialogAccepted entries to ~/.claude.json
// for the given project paths. Returns the number of entries added. Idempotent.
func upsertClaudeJSONTrust(claudeJSONPath string, projectPaths []string) (int, error) {
	if len(projectPaths) == 0 {
		return 0, nil
	}

	var raw map[string]any
	data, rerr := os.ReadFile(claudeJSONPath)
	if rerr != nil && !os.IsNotExist(rerr) {
		return 0, fmt.Errorf("read %s: %w", claudeJSONPath, rerr)
	}
	if rerr == nil {
		if uerr := json.Unmarshal(data, &raw); uerr != nil {
			return 0, fmt.Errorf("parse %s: %w", claudeJSONPath, uerr)
		}
	}
	if raw == nil {
		raw = map[string]any{"hasCompletedOnboarding": true}
	}

	projects, _ := raw["projects"].(map[string]any)
	if projects == nil {
		projects = make(map[string]any)
		raw["projects"] = projects
	}

	added := 0
	for _, path := range projectPaths {
		if proj, exists := projects[path]; exists {
			if m, ok := proj.(map[string]any); ok && m["hasTrustDialogAccepted"] == true {
				continue
			}
		}
		projects[path] = newProjectTrustEntry()
		added++
	}

	if added == 0 {
		return 0, nil
	}

	out, err := json.MarshalIndent(raw, "", "  ")
	if err != nil {
		return 0, fmt.Errorf("marshal %s: %w", claudeJSONPath, err)
	}
	if err := os.WriteFile(claudeJSONPath, out, 0o644); err != nil {
		return 0, fmt.Errorf("write %s: %w", claudeJSONPath, err)
	}
	return added, nil
}

// newProjectTrustEntry returns a trust entry map for a CC project.
func newProjectTrustEntry() map[string]any {
	return map[string]any{
		"hasTrustDialogAccepted":        true,
		"hasCompletedProjectOnboarding": true,
		"allowedTools":                  []any{},
	}
}
