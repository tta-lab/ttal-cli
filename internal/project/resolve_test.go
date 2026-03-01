package project

import (
	"testing"

	"codeberg.org/clawteam/ttal-cli/ent"
)

func TestMatchByContains(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		projects []*ent.Project
		want     string
	}{
		{
			name:  "input contains one alias",
			input: "ttal-cli",
			projects: []*ent.Project{
				{Alias: "ttal", Path: "/path/ttal"},
				{Alias: "flicknote", Path: "/path/flicknote"},
			},
			want: "/path/ttal",
		},
		{
			name:  "input contains multiple aliases - ambiguous",
			input: "ttal-flicknote-app",
			projects: []*ent.Project{
				{Alias: "ttal", Path: "/path/ttal"},
				{Alias: "flicknote", Path: "/path/flicknote"},
			},
			want: "",
		},
		{
			name:  "alias contains input but not vice versa - no match",
			input: "tt",
			projects: []*ent.Project{
				{Alias: "ttal", Path: "/path/ttal"},
			},
			want: "",
		},
		{
			name:  "case insensitive match",
			input: "TTAL-CLI",
			projects: []*ent.Project{
				{Alias: "ttal", Path: "/path/ttal"},
			},
			want: "/path/ttal",
		},
		{
			name:  "empty alias skipped",
			input: "anything",
			projects: []*ent.Project{
				{Alias: "", Path: "/path/empty"},
			},
			want: "",
		},
		{
			name:  "project with no path skipped",
			input: "ttal-cli",
			projects: []*ent.Project{
				{Alias: "ttal", Path: ""},
			},
			want: "",
		},
		{
			name:     "no projects",
			input:    "ttal-cli",
			projects: nil,
			want:     "",
		},
		{
			name:  "exact alias match via contains",
			input: "ttal",
			projects: []*ent.Project{
				{Alias: "ttal", Path: "/path/ttal"},
			},
			want: "/path/ttal",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := matchByContains(tt.input, tt.projects)
			if got != tt.want {
				t.Errorf("matchByContains(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}
