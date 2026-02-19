package watcher

import (
	"testing"
)

func TestEncodePath(t *testing.T) {
	tests := []struct {
		name string
		path string
		want string
	}{
		{"absolute path", "/Users/neil/clawd", "-Users-neil-clawd"},
		{"path with dots", "/Users/neil/my.project", "-Users-neil-my-project"},
		{"root", "/", "-"},
		{"no separators", "simple", "simple"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := encodePath(tt.path)
			if got != tt.want {
				t.Errorf("encodePath(%q) = %q, want %q", tt.path, got, tt.want)
			}
		})
	}
}

func TestSplitCompleteLines(t *testing.T) {
	tests := []struct {
		name  string
		input []byte
		want  int
	}{
		{"empty", []byte{}, 0},
		{"single complete line", []byte("hello\n"), 1},
		{"trailing partial", []byte("hello\nworld"), 1},
		{"two complete lines", []byte("a\nb\n"), 2},
		{"only partial", []byte("no newline"), 0},
		{"empty lines", []byte("\n\n"), 2},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := splitCompleteLines(tt.input)
			if len(got) != tt.want {
				t.Errorf("splitCompleteLines(%q) returned %d lines, want %d", tt.input, len(got), tt.want)
			}
		})
	}
}

func TestExtractAssistantText(t *testing.T) {
	tests := []struct {
		name string
		line string
		want string
	}{
		{
			"assistant with text",
			`{"type":"assistant","message":{"content":[{"type":"text","text":"hello world"}]}}`,
			"hello world",
		},
		{
			"multiple text blocks",
			`{"type":"assistant","message":{"content":[{"type":"text","text":"first"},{"type":"text","text":"second"}]}}`,
			"first\n\nsecond",
		},
		{
			"non-assistant type",
			`{"type":"user","message":{"content":[{"type":"text","text":"hello"}]}}`,
			"",
		},
		{
			"assistant with tool_use only",
			`{"type":"assistant","message":{"content":[{"type":"tool_use","id":"123"}]}}`,
			"",
		},
		{
			"empty text trimmed",
			`{"type":"assistant","message":{"content":[{"type":"text","text":"  "}]}}`,
			"",
		},
		{
			"text gets trimmed",
			`{"type":"assistant","message":{"content":[{"type":"text","text":"  hello  "}]}}`,
			"hello",
		},
		{
			"invalid json",
			`not json`,
			"",
		},
		{
			"empty content array",
			`{"type":"assistant","message":{"content":[]}}`,
			"",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := extractAssistantText([]byte(tt.line))
			if got != tt.want {
				t.Errorf("extractAssistantText() = %q, want %q", got, tt.want)
			}
		})
	}
}
