package sync

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

// DenyPrimaryAgentsAsSubagents reads ~/.claude/settings.json and ensures
// Agent(<name>) deny entries exist for all deployed agents, preventing CC
// from spawning them directly as subagents. All agent routing must go
// through ttal task route.
//
// Additive only — appends new entries at the end of the deny list,
// never removes or reorders existing entries. Returns list of newly added entry names.
func DenyPrimaryAgentsAsSubagents(agentNames []string, dryRun bool) (added []string, err error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil, fmt.Errorf("cannot determine home directory: %w", err)
	}
	settingsPath := filepath.Join(home, ".claude", "settings.json")
	return denyPrimaryAgentsAsSubagents(agentNames, dryRun, settingsPath)
}

func denyPrimaryAgentsAsSubagents(agentNames []string, dryRun bool, settingsPath string) ([]string, error) {
	settings, err := readOrInitSettings(settingsPath)
	if err != nil {
		return nil, err
	}

	perms, denySlice, err := extractPermsDenyList(settings)
	if err != nil {
		return nil, err
	}

	// Build set of existing deny entries for O(1) lookup
	existing := make(map[string]struct{}, len(denySlice))
	for _, v := range denySlice {
		if s, ok := v.(string); ok {
			existing[s] = struct{}{}
		}
	}

	// Append missing Agent(<name>) entries
	var added []string
	for _, name := range agentNames {
		entry := fmt.Sprintf("Agent(%s)", name)
		if _, ok := existing[entry]; ok {
			continue
		}
		denySlice = append(denySlice, entry)
		added = append(added, name)
	}

	if len(added) == 0 {
		return added, nil
	}
	if dryRun {
		return added, nil
	}

	perms["deny"] = denySlice
	settings["permissions"] = perms

	if err := writeSettingsJSON(settingsPath, settings); err != nil {
		return nil, fmt.Errorf("writing settings.json: %w", err)
	}

	return added, nil
}

// readOrInitSettings reads settings.json from path, or returns an empty map if the file doesn't exist.
func readOrInitSettings(settingsPath string) (map[string]interface{}, error) {
	data, err := os.ReadFile(settingsPath)
	if err != nil {
		if os.IsNotExist(err) {
			return map[string]interface{}{}, nil
		}
		return nil, fmt.Errorf("reading settings.json: %w", err)
	}
	var settings map[string]interface{}
	if err := json.Unmarshal(data, &settings); err != nil {
		return nil, fmt.Errorf("parsing settings.json: %w", err)
	}
	return settings, nil
}

// extractPermsDenyList navigates settings to permissions.deny with type validation.
// Returns the permissions map and deny list (both may be empty/nil but are never from bad type assertions).
func extractPermsDenyList(settings map[string]interface{}) (map[string]interface{}, []interface{}, error) {
	var perms map[string]interface{}
	if raw, ok := settings["permissions"]; ok {
		perms, ok = raw.(map[string]interface{})
		if !ok {
			return nil, nil, fmt.Errorf("settings.json: permissions is not an object (got %T)", raw)
		}
	}
	if perms == nil {
		perms = map[string]interface{}{}
	}

	var denySlice []interface{}
	if raw, ok := perms["deny"]; ok {
		denySlice, ok = raw.([]interface{})
		if !ok {
			return nil, nil, fmt.Errorf("settings.json: permissions.deny is not an array (got %T)", raw)
		}
	}

	return perms, denySlice, nil
}

// writeSettingsJSON marshals v to indented JSON and writes to path (0o644).
// Creates parent directory if needed.
func writeSettingsJSON(path string, v interface{}) error {
	data, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	return os.WriteFile(path, append(data, '\n'), 0o644)
}
