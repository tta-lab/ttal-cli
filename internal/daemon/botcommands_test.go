package daemon

import "testing"

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

func TestDiscoverCommands_FiltersStaticAfterSanitize(t *testing.T) {
	// Verify that a skill named "new" (matching static command) gets filtered out.
	// We test this indirectly: isStaticBotCommand("new") should be true.
	if !isStaticBotCommand(sanitizeCommandName("new")) {
		t.Error("expected 'new' to be filtered as static command")
	}
	// A hyphenated name that doesn't collide should pass through.
	if isStaticBotCommand(sanitizeCommandName("review-pr")) {
		t.Error("expected 'review-pr' (sanitized: 'review_pr') to NOT be a static command")
	}
}
