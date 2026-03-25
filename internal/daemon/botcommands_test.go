package daemon

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/tta-lab/ttal-cli/internal/skill"
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

func TestDiscoverCommandsFromRegistry(t *testing.T) {
	dir := t.TempDir()
	registryPath := filepath.Join(dir, "skills.toml")
	if err := os.WriteFile(registryPath, []byte(`
[skills.breathe]
id = "aaa"
category = "command"
description = "Refresh context"

[skills.sp-planning]
id = "bbb"
category = "methodology"
description = "Planning skill"

[skills.new]
id = "ccc"
category = "command"
description = "Should be filtered (static)"

[skills.tell-me-more]
id = "ddd"
category = "command"
description = "Elaborate"
`), 0o644); err != nil {
		t.Fatal(err)
	}

	r, err := skill.Load(registryPath)
	if err != nil {
		t.Fatal(err)
	}

	cmds := discoverCommandsFromRegistry(r)

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
