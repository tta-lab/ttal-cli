package config

import (
	"fmt"
	"log"
	"os"
	"path/filepath"

	"github.com/BurntSushi/toml"
)

type RolesConfig struct {
	Roles              map[string]string `toml:"-"`
	HeartbeatPrompts   map[string]string `toml:"-"`
	HeartbeatIntervals map[string]string `toml:"-"` // per-role heartbeat interval (e.g. "1h")
}

func (r *RolesConfig) UnmarshalTOML(data interface{}) error {
	r.Roles = make(map[string]string)
	r.HeartbeatPrompts = make(map[string]string)
	r.HeartbeatIntervals = make(map[string]string)
	if m, ok := data.(map[string]interface{}); ok {
		for role, v := range m {
			section, ok := v.(map[string]interface{})
			if !ok {
				log.Printf("warning: roles.toml: role [%s] has unexpected type %T, skipping", role, v)
				continue
			}
			if p, ok := section["prompt"].(string); ok {
				r.Roles[role] = p
			} else {
				log.Printf("warning: roles.toml: role [%s] has no valid 'prompt' key, skipping", role)
			}
			if hp, ok := section["heartbeat_prompt"].(string); ok && hp != "" {
				r.HeartbeatPrompts[role] = hp
			}
			if hi, ok := section["heartbeat_interval"].(string); ok && hi != "" {
				r.HeartbeatIntervals[role] = hi
			}
		}
	}
	return nil
}

// HeartbeatIntervalForRole returns the heartbeat interval for a given role.
// Returns empty string if no interval is configured.
func (r *RolesConfig) HeartbeatIntervalForRole(role string) string {
	if r == nil || r.HeartbeatIntervals == nil {
		return ""
	}
	return r.HeartbeatIntervals[role]
}

func LoadRoles() (*RolesConfig, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil, fmt.Errorf("could not determine home directory: %w", err)
	}
	path := filepath.Join(home, ".config", "ttal", "roles.toml")
	var cfg RolesConfig
	if _, err := toml.DecodeFile(path, &cfg); err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	return &cfg, nil
}
