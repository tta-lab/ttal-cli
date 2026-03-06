package config

import (
	"os"
	"path/filepath"

	"github.com/BurntSushi/toml"
)

type RolesConfig struct {
	Roles map[string]string `toml:"-"`
}

func (r *RolesConfig) UnmarshalTOML(data interface{}) error {
	if m, ok := data.(map[string]interface{}); ok {
		r.Roles = make(map[string]string)
		for role, v := range m {
			if section, ok := v.(map[string]interface{}); ok {
				if p, ok := section["prompt"].(string); ok {
					r.Roles[role] = p
				}
			}
		}
	}
	return nil
}

func LoadRoles() (*RolesConfig, error) {
	path := filepath.Join(os.Getenv("HOME"), ".config", "ttal", "roles.toml")
	var cfg RolesConfig
	if _, err := toml.DecodeFile(path, &cfg); err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	return &cfg, nil
}
