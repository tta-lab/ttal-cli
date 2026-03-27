package config

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"

	"github.com/BurntSushi/toml"
)

// SandboxConfig holds per-plane extra paths loaded from sandbox.toml.
// Consumed by sync.SyncSandbox to build the sandbox section in ~/.claude/settings.json.
// Paths support ~ expansion and must include a :ro or :rw suffix.
//
// Enabled controls whether ttal sync writes sandbox enforcement to settings.json.
// All enforcement settings (failIfUnavailable, allowUnsandboxedCommands, network) are
// hardcoded secure defaults in sync — they are not configurable via sandbox.toml.
type SandboxConfig struct {
	Enabled bool         `toml:"enabled"`
	Shared  SandboxPlane `toml:"shared"`
	Worker  SandboxPlane `toml:"worker"`
	Manager SandboxPlane `toml:"manager"`
}

// SandboxPlane holds extra paths for one plane.
type SandboxPlane struct {
	ExtraPaths []string `toml:"extra_paths"`
}

// PathsForPlane returns shared paths merged with the given plane's paths,
// with ~ expanded and non-existent paths filtered out.
// Filtering allows listing both macOS and Linux variants in sandbox.toml —
// only paths present on disk are included.
func (c *SandboxConfig) PathsForPlane(plane string) []string {
	var planeExtra []string
	switch plane {
	case "worker":
		planeExtra = c.Worker.ExtraPaths
	case "manager":
		planeExtra = c.Manager.ExtraPaths
	}

	// Pre-allocate a fresh slice to avoid mutating c.Shared.ExtraPaths backing array
	// when the TOML decoder left spare capacity (classic Go append aliasing trap).
	raw := make([]string, 0, len(c.Shared.ExtraPaths)+len(planeExtra))
	raw = append(raw, c.Shared.ExtraPaths...)
	raw = append(raw, planeExtra...)

	result := make([]string, 0, len(raw))
	for _, p := range raw {
		p = expandHome(p)
		// strip :ro/:rw suffix to stat the bare path
		bare := strings.TrimSuffix(strings.TrimSuffix(p, ":rw"), ":ro")
		if _, err := os.Stat(bare); err != nil {
			continue // skip non-existent paths silently
		}
		result = append(result, p)
	}
	return result
}

// LoadSandbox loads sandbox.toml from the default config dir.
// Returns an empty config (no extra paths) if the file doesn't exist — non-fatal.
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
