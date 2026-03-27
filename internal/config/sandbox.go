package config

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"

	"github.com/BurntSushi/toml"
)

// SandboxConfig holds sandbox path configuration loaded from sandbox.toml.
// Consumed by sync.SyncSandbox to build the sandbox section in ~/.claude/settings.json.
//
// AllowWrite              — paths Claude may write to (maps to sandbox.filesystem.allowWrite).
// DenyWrite               — paths Claude may not write to (maps to sandbox.filesystem.denyWrite).
// DenyRead                — paths Claude may not read (maps to sandbox.filesystem.denyRead).
//                           Typically ["~/"] to deny all home dir reads by default.
// AllowRead               — paths readable within a denied parent (maps to sandbox.filesystem.allowRead).
//                           Used to allowlist specific dirs within the denied home dir.
// PermissionsDeny         — raw permissions.deny entries (e.g. "Read(~/.ssh/id_ed25519)").
//                           Written directly to settings.json permissions.deny (additive, deduplicated).
//                           Use to deny specific secret files within allowRead dirs.
// AutoAllowBashIfSandboxed — auto-approve bash commands when sandboxed (default: true in CC).
//                            Set to false to require approval for each bash command.
// Network                 — network access config (allowed domains, unix sockets are hardcoded by sync).
//
// Enabled controls whether ttal sync writes sandbox enforcement to settings.json.
// failIfUnavailable and allowUnsandboxedCommands are hardcoded secure defaults in sync.
type SandboxConfig struct {
	Enabled                 bool          `toml:"enabled"`
	AutoAllowBashIfSandboxed *bool         `toml:"autoAllowBashIfSandboxed"`
	AllowWrite              []string      `toml:"allowWrite"`
	DenyWrite               []string      `toml:"denyWrite"`
	DenyRead                []string      `toml:"denyRead"`
	AllowRead               []string      `toml:"allowRead"`
	PermissionsDeny         []string      `toml:"permissionsDeny"`
	Network                 NetworkConfig `toml:"network"`
}

// NetworkConfig holds network access settings for the sandbox.
type NetworkConfig struct {
	AllowedDomains []string `toml:"allowedDomains"`
}

// ExpandedAllowWrite returns the AllowWrite paths with ~ expanded.
func (c *SandboxConfig) ExpandedAllowWrite() []string {
	return expandPaths(c.AllowWrite)
}

// ExpandedDenyWrite returns the DenyWrite paths with ~ expanded.
func (c *SandboxConfig) ExpandedDenyWrite() []string {
	return expandPaths(c.DenyWrite)
}

// ExpandedDenyRead returns the DenyRead paths with ~ expanded.
func (c *SandboxConfig) ExpandedDenyRead() []string {
	return expandPaths(c.DenyRead)
}

// ExpandedAllowRead returns the AllowRead paths with ~ expanded.
func (c *SandboxConfig) ExpandedAllowRead() []string {
	return expandPaths(c.AllowRead)
}

// ExpandedPermissionsDeny returns the PermissionsDeny entries with ~ expanded,
// including within Read(...) / Write(...) wrappers.
func (c *SandboxConfig) ExpandedPermissionsDeny() []string {
	return expandPermEntries(c.PermissionsDeny)
}

// expandPaths expands ~ in each path and returns the result.
// Trailing slashes are preserved — "~/" stays as "<home>/" for sandbox denyRead semantics.
func expandPaths(paths []string) []string {
	result := make([]string, 0, len(paths))
	for _, p := range paths {
		expanded := expandHome(p)
		// filepath.Join strips trailing slashes; restore them so that "~/" correctly
		// maps to "<home>/" (CC sandbox uses the trailing slash to match the dir root).
		if strings.HasSuffix(p, "/") && !strings.HasSuffix(expanded, "/") {
			expanded += "/"
		}
		result = append(result, expanded)
	}
	return result
}

// expandPermEntries expands ~ within permissions entries like "Read(~/...)" or bare paths.
// Handles ~ both at string start (bare paths) and inside wrappers like Read(~/...).
func expandPermEntries(entries []string) []string {
	home, err := os.UserHomeDir()
	if err != nil {
		return entries
	}
	result := make([]string, 0, len(entries))
	for _, e := range entries {
		// Replace all occurrences of ~/ and bare ~ (followed by non-/) with expanded home.
		e = strings.ReplaceAll(e, "~/", home+"/")
		if e == "~" {
			e = home
		}
		result = append(result, e)
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
