package sync

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// ttalPluginRef is the plugin@marketplace identifier for claude plugin commands.
const ttalPluginRef = "ttal@ttal"

// PluginResult reports the outcome of a plugin sync operation.
type PluginResult struct {
	MarketplaceAdded bool
	PluginInstalled  bool
	PluginUpdated    bool
	AgentCount       int
}

// InstallPlugin registers the ttal marketplace and installs (or updates) the
// ttal plugin via `claude plugin` CLI commands. repoPath is the root of the
// ttal-cli repo (contains .claude-plugin/marketplace.json).
func InstallPlugin(repoPath string, dryRun bool) (*PluginResult, error) {
	result := &PluginResult{}

	// Count agents in the plugin for reporting.
	agentsDir := filepath.Join(repoPath, "plugin", "agents")
	if entries, err := os.ReadDir(agentsDir); err == nil {
		for _, e := range entries {
			if !e.IsDir() && strings.HasSuffix(e.Name(), ".md") {
				result.AgentCount++
			}
		}
	}

	// Check if marketplace is already registered.
	if !isMarketplaceRegistered(repoPath) {
		if dryRun {
			result.MarketplaceAdded = true
		} else {
			out, err := exec.Command("claude", "plugin", "marketplace", "add", repoPath).CombinedOutput()
			if err != nil {
				return nil, fmt.Errorf("marketplace add failed: %w\n%s", err, out)
			}
			result.MarketplaceAdded = true
		}
	}

	// Check if plugin is already installed.
	installed := isPluginInstalled()

	if !installed {
		if dryRun {
			result.PluginInstalled = true
		} else {
			out, err := exec.Command("claude", "plugin", "install", ttalPluginRef, "--scope", "user").CombinedOutput()
			if err != nil {
				return nil, fmt.Errorf("plugin install failed: %w\n%s", err, out)
			}
			result.PluginInstalled = true
		}
	} else {
		if dryRun {
			result.PluginUpdated = true
		} else {
			out, err := exec.Command("claude", "plugin", "update", ttalPluginRef).CombinedOutput()
			if err != nil {
				// Update can fail if already at latest — not fatal.
				_ = out
			} else {
				result.PluginUpdated = true
			}
		}
	}

	return result, nil
}

// isMarketplaceRegistered checks known_marketplaces.json for the ttal marketplace.
func isMarketplaceRegistered(repoPath string) bool {
	home, err := os.UserHomeDir()
	if err != nil {
		return false
	}
	data, err := os.ReadFile(filepath.Join(home, ".claude", "plugins", "known_marketplaces.json"))
	if err != nil {
		return false
	}
	var markets map[string]interface{}
	if err := json.Unmarshal(data, &markets); err != nil {
		return false
	}
	// The key is the marketplace name from the JSON file.
	for _, v := range markets {
		m, ok := v.(map[string]interface{})
		if !ok {
			continue
		}
		if src, ok := m["source"].(string); ok && src == repoPath {
			return true
		}
	}
	return false
}

// isPluginInstalled checks installed_plugins.json for the ttal plugin.
func isPluginInstalled() bool {
	home, err := os.UserHomeDir()
	if err != nil {
		return false
	}
	data, err := os.ReadFile(filepath.Join(home, ".claude", "plugins", "installed_plugins.json"))
	if err != nil {
		return false
	}
	var installed struct {
		Plugins map[string]interface{} `json:"plugins"`
	}
	if err := json.Unmarshal(data, &installed); err != nil {
		return false
	}
	_, ok := installed.Plugins[ttalPluginRef]
	return ok
}
