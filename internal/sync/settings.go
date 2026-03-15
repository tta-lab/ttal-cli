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

func denyPrimaryAgentsAsSubagents(agentNames []string, dryRun bool, settingsPath string) (added []string, err error) {
	var settings map[string]interface{}

	data, err := os.ReadFile(settingsPath)
	if err != nil {
		if !os.IsNotExist(err) {
			return nil, fmt.Errorf("reading settings.json: %w", err)
		}
		settings = map[string]interface{}{}
	} else {
		if err := json.Unmarshal(data, &settings); err != nil {
			return nil, fmt.Errorf("parsing settings.json: %w", err)
		}
	}

	// Navigate to permissions.deny, creating maps/slices as needed
	perms, _ := settings["permissions"].(map[string]interface{})
	if perms == nil {
		perms = map[string]interface{}{}
	}

	var denySlice []interface{}
	if raw, ok := perms["deny"]; ok {
		denySlice, _ = raw.([]interface{})
	}

	// Build set of existing deny entries for O(1) lookup
	existing := make(map[string]struct{}, len(denySlice))
	for _, v := range denySlice {
		if s, ok := v.(string); ok {
			existing[s] = struct{}{}
		}
	}

	// Append missing Agent(<name>) entries
	for _, name := range agentNames {
		entry := fmt.Sprintf("Agent(%s)", name)
		if _, ok := existing[entry]; ok {
			continue
		}
		denySlice = append(denySlice, entry)
		added = append(added, name)
	}

	if len(added) == 0 || dryRun {
		return added, nil
	}

	perms["deny"] = denySlice
	settings["permissions"] = perms

	if err := writeSettingsJSON(settingsPath, settings); err != nil {
		return nil, fmt.Errorf("writing settings.json: %w", err)
	}

	return added, nil
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
