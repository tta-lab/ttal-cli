package cmd

import (
	"strings"
	"testing"
)

func TestExtractIDFromReader(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{"valid JSON with string id", `{"id":"abc123","project":"foo"}`, "abc123"},
		{"valid JSON with non-string id", `{"id":42}`, ""},
		{"valid JSON without id", `{"project":"foo"}`, ""},
		{"invalid JSON", `not json`, ""},
		{"empty input", ``, ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := extractIDFromReader(strings.NewReader(tt.input))
			if got != tt.want {
				t.Errorf("extractIDFromReader(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}
