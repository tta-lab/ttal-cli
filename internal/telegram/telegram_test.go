package telegram

import (
	"strings"
	"testing"

	"github.com/go-telegram/bot/models"
)

func TestSplitMessage_Short(t *testing.T) {
	text := "hello world"
	parts := renderMessageTexts(text)
	if len(parts) != 1 || parts[0] != text {
		t.Errorf("expected single chunk %q, got %v", text, parts)
	}
}

func TestSplitMessage_Exact4096(t *testing.T) {
	text := strings.Repeat("a", 4096)
	parts := renderMessageTexts(text)
	if len(parts) != 1 {
		t.Errorf("expected 1 chunk, got %d", len(parts))
	}
}

func TestSplitMessage_ParagraphBreak(t *testing.T) {
	half := strings.Repeat("a", 2500)
	text := half + "\n\n" + half
	parts := renderMessageTexts(text)
	if len(parts) != 2 {
		t.Errorf("expected 2 chunks, got %d", len(parts))
	}
	for _, p := range parts {
		if len([]rune(p)) > maxMessageLen {
			t.Errorf("chunk too long: %d chars", len([]rune(p)))
		}
	}
}

func TestSplitMessage_SingleNewline(t *testing.T) {
	line := strings.Repeat("a", 100) + "\n"
	text := strings.Repeat(line, 50) // 5050 chars
	parts := renderMessageTexts(text)
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
	parts := renderMessageTexts(text)
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
	parts := renderMessageTexts(text)
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
	const textLen = 5000
	text := strings.Repeat("a", textLen)
	parts := renderMessageTexts(text)
	if len(parts) != 2 {
		t.Errorf("expected 2 chunks, got %d", len(parts))
	}
	if len(parts[0]) != maxMessageLen {
		t.Errorf("expected first chunk to be %d, got %d", maxMessageLen, len(parts[0]))
	}
	want := textLen - maxMessageLen
	if len(parts[1]) != want {
		t.Errorf("expected second chunk to be %d, got %d", want, len(parts[1]))
	}
}

func TestSplitMessage_Empty(t *testing.T) {
	parts := renderMessageTexts("")
	if len(parts) != 1 {
		t.Errorf("expected 1 part for empty string, got %d", len(parts))
	}
}

func TestSplitMessage_AllWhitespaceOverLimit(t *testing.T) {
	text := strings.Repeat("\n", 5000)
	parts := renderMessageTexts(text)
	// verifies no panic or infinite loop; all whitespace trims to nothing so
	// rendering returns an empty slice. SendMessage's TrimSpace guard prevents
	// this input from ever reaching renderMessageChunks in practice.
	_ = parts
}

func TestSplitMessage_NonASCII(t *testing.T) {
	// Each emoji is 4 bytes — 4096 runes = 16384 bytes.
	// Byte-based slicing would corrupt at char boundaries; rune-based is safe.
	text := strings.Repeat("😀", 5000)
	parts := renderMessageTexts(text)
	if len(parts) < 2 {
		t.Errorf("expected at least 2 chunks, got %d", len(parts))
	}
	for _, p := range parts {
		if len([]rune(p)) > maxMessageLen {
			t.Errorf("chunk exceeds rune limit: %d runes", len([]rune(p)))
		}
	}
}

func renderMessageTexts(text string) []string {
	chunks := renderMessageChunks(text)
	parts := make([]string, 0, len(chunks))
	for _, chunk := range chunks {
		parts = append(parts, chunk.Text)
	}
	return parts
}

func TestRenderMessageChunks_MarkdownEntities(t *testing.T) {
	chunks := renderMessageChunks("hello **world**")
	if len(chunks) != 1 {
		t.Fatalf("expected 1 chunk, got %d", len(chunks))
	}
	if chunks[0].Text != "hello world" {
		t.Fatalf("text = %q, want %q", chunks[0].Text, "hello world")
	}
	if len(chunks[0].Entities) != 1 {
		t.Fatalf("expected 1 entity, got %d", len(chunks[0].Entities))
	}
	if chunks[0].Entities[0].Type != models.MessageEntityTypeBold {
		t.Errorf("entity type = %q, want %q", chunks[0].Entities[0].Type, models.MessageEntityTypeBold)
	}
	if chunks[0].Entities[0].Offset != 6 || chunks[0].Entities[0].Length != 5 {
		t.Errorf("entity span = (%d, %d), want (6, 5)", chunks[0].Entities[0].Offset, chunks[0].Entities[0].Length)
	}
}

func TestRenderMessageChunks_SplitsMarkdown(t *testing.T) {
	chunks := renderMessageChunks(strings.Repeat("**bold** ", 1000))
	if len(chunks) < 2 {
		t.Fatalf("expected multiple chunks, got %d", len(chunks))
	}
	for _, chunk := range chunks {
		if len([]rune(chunk.Text)) > maxMessageLen {
			t.Errorf("chunk exceeds max length: %d", len([]rune(chunk.Text)))
		}
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
