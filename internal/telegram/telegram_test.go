package telegram

import (
	"strings"
	"testing"
)

func TestSplitMessage(t *testing.T) {
	t.Run("short message returns single chunk", func(t *testing.T) {
		text := "hello world"
		parts := splitMessage(text)
		if len(parts) != 1 || parts[0] != text {
			t.Errorf("expected single chunk %q, got %v", text, parts)
		}
	})

	t.Run("exact 4096 chars returns single chunk", func(t *testing.T) {
		text := strings.Repeat("a", 4096)
		parts := splitMessage(text)
		if len(parts) != 1 {
			t.Errorf("expected 1 chunk, got %d", len(parts))
		}
	})

	t.Run("5000 chars with paragraph breaks splits at double newline", func(t *testing.T) {
		half := strings.Repeat("a", 2500)
		text := half + "\n\n" + half
		parts := splitMessage(text)
		if len(parts) != 2 {
			t.Errorf("expected 2 chunks, got %d: %v", len(parts), parts)
		}
		for _, p := range parts {
			if strings.HasPrefix(p, "\n") || strings.HasSuffix(p, "\n") {
				t.Errorf("chunk has leading/trailing newline: %q", p)
			}
		}
	})

	t.Run("5000 chars with only single newlines splits at newline", func(t *testing.T) {
		// build text with single newlines (no double), long enough to need splitting
		line := strings.Repeat("a", 100) + "\n"
		text := strings.Repeat(line, 50) // 5050 chars
		parts := splitMessage(text)
		if len(parts) < 2 {
			t.Errorf("expected at least 2 chunks, got %d", len(parts))
		}
		for _, p := range parts {
			if len(p) > maxMessageLen {
				t.Errorf("chunk too long: %d chars", len(p))
			}
		}
	})

	t.Run("5000 chars with only spaces splits at space", func(t *testing.T) {
		word := strings.Repeat("a", 50) + " "
		text := strings.Repeat(word, 98) // ~4998 chars, no newlines
		parts := splitMessage(text)
		if len(parts) < 2 {
			t.Errorf("expected at least 2 chunks, got %d", len(parts))
		}
		for _, p := range parts {
			if len(p) > maxMessageLen {
				t.Errorf("chunk too long: %d chars", len(p))
			}
		}
	})

	t.Run("8000+ chars produces multiple chunks all within limit", func(t *testing.T) {
		text := strings.Repeat("x\n", 4001) // 8002 chars
		parts := splitMessage(text)
		if len(parts) < 2 {
			t.Errorf("expected multiple chunks, got %d", len(parts))
		}
		for _, p := range parts {
			if len(p) > maxMessageLen {
				t.Errorf("chunk too long: %d chars", len(p))
			}
		}
	})

	t.Run("no natural boundaries hard cuts at 4096", func(t *testing.T) {
		text := strings.Repeat("a", 5000)
		parts := splitMessage(text)
		if len(parts) != 2 {
			t.Errorf("expected 2 chunks, got %d", len(parts))
		}
		if len(parts[0]) != maxMessageLen {
			t.Errorf("expected first chunk to be %d, got %d", maxMessageLen, len(parts[0]))
		}
		if len(parts[1]) != 904 {
			t.Errorf("expected second chunk to be 904, got %d", len(parts[1]))
		}
	})

	t.Run("empty input returns empty slice", func(t *testing.T) {
		parts := splitMessage("")
		// empty string length is 0 <= maxMessageLen, returns []string{""}
		// but we don't want to send empty messages — callers handle this
		// splitMessage returns single empty string for empty input (matches short-circuit)
		if len(parts) != 1 {
			t.Errorf("expected 1 part for empty string, got %d", len(parts))
		}
	})
}

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
		{"ttal:send", "🕊"},
		{"ttal:route", "🕊"},
		{"flicknote:write", "✍"},
		{"flicknote:read", "👀"},
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
