package telegram

import (
	"strings"
	"testing"
)

func TestSplitMessage_Short(t *testing.T) {
	text := "hello world"
	parts := splitMessage(text)
	if len(parts) != 1 || parts[0] != text {
		t.Errorf("expected single chunk %q, got %v", text, parts)
	}
}

func TestSplitMessage_Exact4096(t *testing.T) {
	text := strings.Repeat("a", 4096)
	parts := splitMessage(text)
	if len(parts) != 1 {
		t.Errorf("expected 1 chunk, got %d", len(parts))
	}
}

func TestSplitMessage_ParagraphBreak(t *testing.T) {
	half := strings.Repeat("a", 2500)
	text := half + "\n\n" + half
	parts := splitMessage(text)
	if len(parts) != 2 {
		t.Errorf("expected 2 chunks, got %d", len(parts))
	}
	for _, p := range parts {
		if strings.HasPrefix(p, "\n") || strings.HasSuffix(p, "\n") {
			t.Errorf("chunk has leading/trailing newline: %q", p)
		}
	}
}

func TestSplitMessage_SingleNewline(t *testing.T) {
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
}

func TestSplitMessage_Space(t *testing.T) {
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
}

func TestSplitMessage_MultiChunk(t *testing.T) {
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
}

func TestSplitMessage_HardCut(t *testing.T) {
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
}

func TestSplitMessage_Empty(t *testing.T) {
	parts := splitMessage("")
	if len(parts) != 1 {
		t.Errorf("expected 1 part for empty string, got %d", len(parts))
	}
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
