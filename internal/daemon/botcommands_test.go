package daemon

import (
	"testing"
)

func TestSanitizeCommandName(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"review-pr", "review_pr"},
		{"status", "status"},
		{"multi-hyphen-name", "multi_hyphen_name"},
		{"already_underscored", "already_underscored"},
		{"", ""},
		{"a", "a"},
		{"-leading", "_leading"},
		{"trailing-", "trailing_"},
	}
	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := sanitizeCommandName(tt.input)
			if got != tt.want {
				t.Errorf("sanitizeCommandName(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestIsStaticCommand(t *testing.T) {
	tests := []struct {
		name string
		want bool
	}{
		{"status", true},
		{"new", true},
		{"compact", true},
		{"wait", true},
		{"help", true},
		{"save", true},
		{"review_pr", false},
		{"unknown", false},
		{"", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := isStaticBotCommand(tt.name)
			if got != tt.want {
				t.Errorf("isStaticBotCommand(%q) = %v, want %v", tt.name, got, tt.want)
			}
		})
	}
}

func TestDiscoverCommandsFromSkills(t *testing.T) {
	// Test: mix of command/non-command skills → only commands returned
	skills := []skillEntry{
		{Name: "breathe", Category: "command", Description: "Refresh context"},
		{Name: "sp-planning", Category: "methodology", Description: "Planning skill"},
		{Name: "new", Category: "command", Description: "Should be filtered (static)"},
		{Name: "tell-me-more", Category: "command", Description: "Elaborate"},
	}

	cmds := discoverCommandsFromSkills(skills)

	// Should find breathe and tell-me-more (commands), skip sp-planning (methodology) and new (static).
	if len(cmds) != 2 {
		t.Fatalf("expected 2 commands, got %d: %+v", len(cmds), cmds)
	}

	names := map[string]bool{}
	for _, c := range cmds {
		names[c.OriginalName] = true
	}
	if !names["breathe"] || !names["tell-me-more"] {
		t.Errorf("missing expected commands: %v", names)
	}
	if names["new"] {
		t.Error("static command 'new' should be filtered")
	}
	// Verify tell-me-more is sanitized to tell_me_more
	for _, c := range cmds {
		if c.OriginalName == "tell-me-more" && c.Command != "tell_me_more" {
			t.Errorf("expected sanitized command tell_me_more, got %s", c.Command)
		}
	}
}
