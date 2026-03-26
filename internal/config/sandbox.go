package config

import (
	"log"
	"os"
	"path/filepath"
	"strings"

	"github.com/BurntSushi/toml"
)

// SandboxConfig holds per-plane extra TEMENOS_PATHS loaded from sandbox.toml.
// Paths support ~ expansion and must include a :ro or :rw suffix.
type SandboxConfig struct {
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
	raw := append(c.Shared.ExtraPaths, c.planeSection(plane).ExtraPaths...)
	home, _ := os.UserHomeDir()
	result := make([]string, 0, len(raw))
	for _, p := range raw {
		if len(p) >= 1 && p[0] == '~' {
			p = home + p[1:]
		}
		// strip :ro/:rw suffix to stat the bare path
		bare := strings.TrimSuffix(strings.TrimSuffix(p, ":rw"), ":ro")
		if _, err := os.Stat(bare); err != nil {
			continue // skip non-existent paths silently
		}
		result = append(result, p)
	}
	return result
}

func (c *SandboxConfig) planeSection(plane string) SandboxPlane {
	switch plane {
	case "worker":
		return c.Worker
	case "manager":
		return c.Manager
	default:
		return SandboxPlane{}
	}
}

// LoadSandbox loads sandbox.toml from the default config dir.
// Returns an empty config (no extra paths) if the file doesn't exist — non-fatal.
func LoadSandbox() *SandboxConfig {
	path := filepath.Join(DefaultConfigDir(), "sandbox.toml")
	var cfg SandboxConfig
	if _, err := os.Stat(path); os.IsNotExist(err) {
		return &cfg
	}
	if _, err := toml.DecodeFile(path, &cfg); err != nil {
		log.Printf("[config] warning: failed to load sandbox.toml: %v", err)
	}
	return &cfg
}
