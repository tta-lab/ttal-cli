package config

import (
	"fmt"
	"log"
	"os"
	"path/filepath"

	"github.com/BurntSushi/toml"
)

// SandboxConfig holds sandbox path configuration loaded from sandbox.toml.
// Consumed by sync.SyncSandbox to build the sandbox section in ~/.claude/settings.json.
//
// AllowWrite — paths Claude may write to (maps to sandbox.filesystem.allowWrite).
// DenyRead   — paths Claude may not read (maps to sandbox.filesystem.denyRead and permissions.deny).
// AllowRead  — paths readable within a denied parent (maps to sandbox.filesystem.allowRead).
// Network    — network access config (allowed domains, unix sockets are hardcoded by sync).
//
// Enabled controls whether ttal sync writes sandbox enforcement to settings.json.
// All enforcement settings (failIfUnavailable, allowUnsandboxedCommands) are
// hardcoded secure defaults in sync — they are not configurable via sandbox.toml.
type SandboxConfig struct {
	Enabled    bool          `toml:"enabled"`
	AllowWrite []string      `toml:"allowWrite"`
	DenyRead   []string      `toml:"denyRead"`
	AllowRead  []string      `toml:"allowRead"`
	Network    NetworkConfig `toml:"network"`
}

// NetworkConfig holds network access settings for the sandbox.
type NetworkConfig struct {
	AllowedDomains []string `toml:"allowedDomains"`
}

// ExpandedAllowWrite returns the AllowWrite paths with ~ expanded.
func (c *SandboxConfig) ExpandedAllowWrite() []string {
	return expandPaths(c.AllowWrite)
}

// ExpandedDenyRead returns the DenyRead paths with ~ expanded.
func (c *SandboxConfig) ExpandedDenyRead() []string {
	return expandPaths(c.DenyRead)
}

// ExpandedAllowRead returns the AllowRead paths with ~ expanded.
func (c *SandboxConfig) ExpandedAllowRead() []string {
	return expandPaths(c.AllowRead)
}

// expandPaths expands ~ in each path and returns the result.
func expandPaths(paths []string) []string {
	result := make([]string, 0, len(paths))
	for _, p := range paths {
		result = append(result, expandHome(p))
	}
	return result
}

// LoadSandbox loads sandbox.toml from the default config dir.
// Returns an empty config (no paths) if the file doesn't exist — non-fatal.
// Parse errors are logged as warnings and return an empty config.
// Use LoadSandboxWithError when parse failures must be surfaced (e.g. security-critical paths).
func LoadSandbox() *SandboxConfig {
	cfg, err := LoadSandboxWithError()
	if err != nil {
		log.Printf("[config] warning: %v", err)
		return &SandboxConfig{}
	}
	return cfg
}

// LoadSandboxWithError loads sandbox.toml from the default config dir.
// Returns an empty config (Enabled: false) when the file is absent — treated as disabled.
// Returns an error when the file exists but cannot be read or parsed, so callers can
// distinguish "disabled by config" from "broken config".
func LoadSandboxWithError() (*SandboxConfig, error) {
	path := filepath.Join(DefaultConfigDir(), "sandbox.toml")
	if _, err := os.Stat(path); err != nil {
		if os.IsNotExist(err) {
			return &SandboxConfig{}, nil
		}
		return nil, fmt.Errorf("cannot stat sandbox.toml: %w", err)
	}
	var cfg SandboxConfig
	if _, err := toml.DecodeFile(path, &cfg); err != nil {
		return nil, fmt.Errorf("failed to parse sandbox.toml: %w", err)
	}
	return &cfg, nil
}
