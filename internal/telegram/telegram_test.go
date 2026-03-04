package telegram

import "testing"

func TestToolEmoji(t *testing.T) {
	tests := []struct {
		tool  string
		emoji string
	}{
		{"Read", "🤔"},
		{"Glob", "🤔"},
		{"Grep", "🤔"},
		{"Edit", "✍"},
		{"Write", "✍"},
		{"Bash", "👨‍💻"},
		{"WebSearch", "👀"},
		{"WebFetch", "👀"},
		{"Agent", "🔥"},
		{"AskUserQuestion", ""},
		{"SomeUnknownTool", "🔥"},
	}

	for _, tt := range tests {
		t.Run(tt.tool, func(t *testing.T) {
			got := ToolEmoji(tt.tool)
			if got != tt.emoji {
				t.Errorf("ToolEmoji(%q) = %q, want %q", tt.tool, got, tt.emoji)
			}
		})
	}
}
