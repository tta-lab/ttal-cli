package config

import (
	"fmt"
	"log"
	"os"
	"path/filepath"

	"github.com/BurntSushi/toml"
)

type RolesConfig struct {
	Roles              map[string]string   `toml:"-"`
	HeartbeatPrompts   map[string]string   `toml:"-"`
	HeartbeatIntervals map[string]string   `toml:"-"` // per-role heartbeat interval (e.g. "1h")
	DefaultSkills      []string            `toml:"default_skills"`
	ExtraSkills        map[string][]string `toml:"extra_skills"` // per-role extra skills
}

//nolint:gocyclo
func (r *RolesConfig) UnmarshalTOML(data interface{}) error {
	r.Roles = make(map[string]string)
	r.HeartbeatPrompts = make(map[string]string)
	r.HeartbeatIntervals = make(map[string]string)
	r.ExtraSkills = make(map[string][]string)
	if m, ok := data.(map[string]interface{}); ok {
		// Top-level default_skills (outside any role block).
		if ds, ok := m["default_skills"].([]interface{}); ok {
			for _, v := range ds {
				if s, ok := v.(string); ok {
					r.DefaultSkills = append(r.DefaultSkills, s)
				}
			}
		}
		for role, v := range m {
			// Skip top-level keys that are not role blocks.
			if role == "default_skills" {
				continue
			}
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
			if es, ok := section["extra_skills"].([]interface{}); ok {
				var skills []string
				for _, v := range es {
					if s, ok := v.(string); ok {
						skills = append(skills, s)
					}
				}
				r.ExtraSkills[role] = skills
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

// RoleSkills returns the effective skill list for a role.
// It merges DefaultSkills with ExtraSkills[role], deduplicating in order.
// Returns DefaultSkills (never nil) for unknown roles.
func (r *RolesConfig) RoleSkills(role string) []string {
	if r == nil {
		return nil
	}
	var result []string
	seen := make(map[string]bool)
	appendUnique := func(s string) {
		if s != "" && !seen[s] {
			seen[s] = true
			result = append(result, s)
		}
	}
	for _, s := range r.DefaultSkills {
		appendUnique(s)
	}
	if extra, ok := r.ExtraSkills[role]; ok {
		for _, s := range extra {
			appendUnique(s)
		}
	}
	if len(result) == 0 {
		return append([]string(nil), r.DefaultSkills...)
	}
	return result
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
