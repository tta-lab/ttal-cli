package config

import (
	"fmt"
	"log"
	"os"
	"path/filepath"

	"github.com/BurntSushi/toml"
)

type RolesConfig struct {
	Roles            map[string]string `toml:"-"`
	HeartbeatPrompts map[string]string `toml:"-"`
}

func (r *RolesConfig) UnmarshalTOML(data interface{}) error {
	r.Roles = make(map[string]string)
	r.HeartbeatPrompts = make(map[string]string)
	if m, ok := data.(map[string]interface{}); ok {
		for role, v := range m {
			if section, ok := v.(map[string]interface{}); ok {
				if p, ok := section["prompt"].(string); ok {
					r.Roles[role] = p
				} else {
					log.Printf("warning: roles.toml: role [%s] has no valid 'prompt' key, skipping", role)
				}
				if hp, ok := section["heartbeat_prompt"].(string); ok && hp != "" {
					r.HeartbeatPrompts[role] = hp
				}
			}
		}
	}
	return nil
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
