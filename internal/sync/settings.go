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
// through ttal go.
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

// ttalContextCommand is the command used in the SessionStart hook.
const ttalContextCommand = "ttal context"

// sessionStartHookEntry is a single entry in a SessionStart hooks list.
type sessionStartHookEntry struct {
	Type    string `json:"type"`
	Command string `json:"command"`
	Timeout int    `json:"timeout"`
}

// sessionStartMatcher is one element of the SessionStart array.
type sessionStartMatcher struct {
	Matcher string                  `json:"matcher"`
	Hooks   []sessionStartHookEntry `json:"hooks"`
}

// InstallSessionStartHook writes a SessionStart entry into ~/.claude/settings.json.
// Additive: if the key exists, adds our entry only if a hook with "ttal context" is absent.
// Preserves all existing non-ttal SessionStart hooks.
// Returns true if the hook was added (false if already present or dry-run with no change).
func InstallSessionStartHook(dryRun bool) (added bool, err error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return false, fmt.Errorf("cannot determine home directory: %w", err)
	}
	settingsPath := filepath.Join(home, ".claude", "settings.json")
	return installSessionStartHook(dryRun, settingsPath)
}

func installSessionStartHook(dryRun bool, settingsPath string) (bool, error) {
	settings, err := readOrInitSettings(settingsPath)
	if err != nil {
		return false, err
	}

	// Read existing SessionStart value if present.
	existing := make([]interface{}, 0, 1)
	if raw, ok := settings["SessionStart"]; ok {
		existing, ok = raw.([]interface{})
		if !ok {
			return false, fmt.Errorf("settings.json: SessionStart is not an array (got %T)", raw)
		}
	}

	// Check if our hook is already present.
	for _, entry := range existing {
		m, ok := entry.(map[string]interface{})
		if !ok {
			continue
		}
		hooks, ok := m["hooks"].([]interface{})
		if !ok {
			continue
		}
		for _, h := range hooks {
			hm, ok := h.(map[string]interface{})
			if !ok {
				continue
			}
			if cmd, ok := hm["command"].(string); ok && cmd == ttalContextCommand {
				return false, nil // already present
			}
		}
	}

	// Not present — append our matcher.
	newEntry := sessionStartMatcher{
		Matcher: "*",
		Hooks: []sessionStartHookEntry{
			{Type: "command", Command: ttalContextCommand, Timeout: 15},
		},
	}
	existing = append(existing, newEntry)

	if dryRun {
		return true, nil
	}

	settings["SessionStart"] = existing
	if err := writeSettingsJSON(settingsPath, settings); err != nil {
		return false, fmt.Errorf("writing settings.json: %w", err)
	}
	return true, nil
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
