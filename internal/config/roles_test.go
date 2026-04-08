package config

import (
	"testing"
)

func TestRolesConfig_RoleSkills(t *testing.T) {
	tests := []struct {
		name         string
		cfg          *RolesConfig
		role         string
		wantLen      int
		wantContains []string
	}{
		{
			name:    "nil config returns nil",
			cfg:     nil,
			role:    "designer",
			wantLen: 0,
		},
		{
			name: "unknown role returns default_skills",
			cfg: &RolesConfig{
				DefaultSkills: []string{"task-tree", "flicknote", "ei-ask"},
				ExtraSkills: map[string][]string{
					"designer": {"sp-planning"},
				},
			},
			role:         "unknown-role",
			wantLen:      3,
			wantContains: []string{"task-tree", "flicknote", "ei-ask"},
		},
		{
			name: "designer gets default plus extra_skills deduped",
			cfg: &RolesConfig{
				DefaultSkills: []string{"task-tree", "flicknote", "ei-ask"},
				ExtraSkills: map[string][]string{
					"designer": {"sp-planning"},
				},
			},
			role:         "designer",
			wantLen:      4,
			wantContains: []string{"task-tree", "flicknote", "ei-ask", "sp-planning"},
		},
		{
			name: "fixer gets sp-debugging extra",
			cfg: &RolesConfig{
				DefaultSkills: []string{"task-tree", "flicknote", "ei-ask"},
				ExtraSkills: map[string][]string{
					"fixer": {"sp-debugging"},
				},
			},
			role:         "fixer",
			wantLen:      4,
			wantContains: []string{"task-tree", "flicknote", "ei-ask", "sp-debugging"},
		},
		{
			name: "empty default_skills returns extra only",
			cfg: &RolesConfig{
				DefaultSkills: []string{},
				ExtraSkills: map[string][]string{
					"auditor": {"sp-research"},
				},
			},
			role:         "auditor",
			wantLen:      1,
			wantContains: []string{"sp-research"},
		},
		{
			name: "no extra_skills for role returns default only",
			cfg: &RolesConfig{
				DefaultSkills: []string{"task-tree", "flicknote", "ei-ask"},
				ExtraSkills:   map[string][]string{},
			},
			role:         "manager",
			wantLen:      3,
			wantContains: []string{"task-tree", "flicknote", "ei-ask"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.cfg.RoleSkills(tt.role)
			if tt.cfg == nil {
				// nil config returns nil
				if got != nil {
					t.Errorf("RoleSkills() on nil config = %v, want nil", got)
				}
				return
			}
			if len(got) != tt.wantLen {
				t.Errorf("RoleSkills(%q) len = %d, want %d; got %v", tt.role, len(got), tt.wantLen, got)
			}
			for _, want := range tt.wantContains {
				found := false
				for _, s := range got {
					if s == want {
						found = true
						break
					}
				}
				if !found {
					t.Errorf("RoleSkills(%q) missing expected skill %q; got %v", tt.role, want, got)
				}
			}
		})
	}
}
