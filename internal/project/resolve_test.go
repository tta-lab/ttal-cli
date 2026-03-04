package project

import (
	"testing"
)

func TestMatchByContains(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		projects []Project
		want     string
	}{
		{
			name:  "input contains one alias",
			input: "ttal-cli",
			projects: []Project{
				{Alias: "ttal", Path: "/path/ttal"},
				{Alias: "flicknote", Path: "/path/flicknote"},
			},
			want: "/path/ttal",
		},
		{
			name:  "input contains multiple aliases - ambiguous",
			input: "ttal-flicknote-app",
			projects: []Project{
				{Alias: "ttal", Path: "/path/ttal"},
				{Alias: "flicknote", Path: "/path/flicknote"},
			},
			want: "",
		},
		{
			name:  "alias contains input but not vice versa - no match",
			input: "tt",
			projects: []Project{
				{Alias: "ttal", Path: "/path/ttal"},
			},
			want: "",
		},
		{
			name:  "case insensitive match",
			input: "TTAL-CLI",
			projects: []Project{
				{Alias: "ttal", Path: "/path/ttal"},
			},
			want: "/path/ttal",
		},
		{
			name:  "empty alias skipped",
			input: "anything",
			projects: []Project{
				{Alias: "", Path: "/path/empty"},
			},
			want: "",
		},
		{
			name:  "project with no path skipped",
			input: "ttal-cli",
			projects: []Project{
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
			projects: []Project{
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
