package bridge

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

// settingsPath returns ~/.claude/settings.json.
func settingsPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".claude", "settings.json"), nil
}

// hookEntry is the structure CC expects for a single hook command.
type hookEntry struct {
	Type    string `json:"type"`
	Command string `json:"command"`
	Timeout int    `json:"timeout"`
}

// hookGroup wraps a list of hook commands (CC's nested format).
type hookGroup struct {
	Hooks []hookEntry `json:"hooks"`
}

// Install adds the ttal bridge Stop hook to ~/.claude/settings.json.
func Install() error {
	path, err := settingsPath()
	if err != nil {
		return err
	}

	settings, err := readSettings(path)
	if err != nil {
		return err
	}

	// Ensure hooks.Stop exists
	hooks, _ := settings["hooks"].(map[string]any)
	if hooks == nil {
		hooks = make(map[string]any)
		settings["hooks"] = hooks
	}

	stopHooks, _ := hooks["Stop"].([]any)

	// Check if ttal bridge hook already exists
	for _, group := range stopHooks {
		groupMap, ok := group.(map[string]any)
		if !ok {
			continue
		}
		innerHooks, _ := groupMap["hooks"].([]any)
		for _, h := range innerHooks {
			hMap, ok := h.(map[string]any)
			if !ok {
				continue
			}
			if cmd, _ := hMap["command"].(string); cmd == "ttal bridge" {
				fmt.Println("Stop hook already installed.")
				return nil
			}
		}
	}

	// Add the hook
	newGroup := map[string]any{
		"hooks": []any{
			map[string]any{
				"type":    "command",
				"command": "ttal bridge",
				"timeout": 5,
			},
		},
	}
	stopHooks = append(stopHooks, newGroup)
	hooks["Stop"] = stopHooks

	if err := writeSettings(path, settings); err != nil {
		return err
	}

	fmt.Printf("Stop hook installed in %s\n", path)
	return nil
}

// Uninstall removes the ttal bridge Stop hook from ~/.claude/settings.json.
func Uninstall() error {
	path, err := settingsPath()
	if err != nil {
		return err
	}

	settings, err := readSettings(path)
	if err != nil {
		return err
	}

	hooks, _ := settings["hooks"].(map[string]any)
	if hooks == nil {
		fmt.Println("No hooks configured.")
		return nil
	}

	stopHooks, _ := hooks["Stop"].([]any)
	if len(stopHooks) == 0 {
		fmt.Println("No Stop hooks configured.")
		return nil
	}

	// Filter out groups containing ttal bridge
	var filtered []any
	removed := false
	for _, group := range stopHooks {
		groupMap, ok := group.(map[string]any)
		if !ok {
			filtered = append(filtered, group)
			continue
		}
		innerHooks, _ := groupMap["hooks"].([]any)
		var keep []any
		for _, h := range innerHooks {
			hMap, ok := h.(map[string]any)
			if !ok {
				keep = append(keep, h)
				continue
			}
			if cmd, _ := hMap["command"].(string); cmd == "ttal bridge" {
				removed = true
				continue
			}
			keep = append(keep, h)
		}
		if len(keep) > 0 {
			groupMap["hooks"] = keep
			filtered = append(filtered, groupMap)
		}
	}

	if !removed {
		fmt.Println("Stop hook not found.")
		return nil
	}

	if len(filtered) == 0 {
		delete(hooks, "Stop")
	} else {
		hooks["Stop"] = filtered
	}

	if len(hooks) == 0 {
		delete(settings, "hooks")
	}

	if err := writeSettings(path, settings); err != nil {
		return err
	}

	fmt.Printf("Stop hook removed from %s\n", path)
	return nil
}

// readSettings reads and parses ~/.claude/settings.json, returning an empty map if not found.
func readSettings(path string) (map[string]any, error) {
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return make(map[string]any), nil
	}
	if err != nil {
		return nil, fmt.Errorf("read settings: %w", err)
	}

	var settings map[string]any
	if err := json.Unmarshal(data, &settings); err != nil {
		return nil, fmt.Errorf("parse settings: %w", err)
	}
	return settings, nil
}

// writeSettings writes settings back to ~/.claude/settings.json preserving other fields.
func writeSettings(path string, settings map[string]any) error {
	data, err := json.MarshalIndent(settings, "", "    ")
	if err != nil {
		return fmt.Errorf("marshal settings: %w", err)
	}

	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}

	return os.WriteFile(path, append(data, '\n'), 0o644)
}
