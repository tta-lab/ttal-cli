package tui

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestDeleteLastWord(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"hello world", "hello"},
		{"hello", ""},
		{"", ""},
		{"hello  ", ""},
		{"one two three", "one two"},
	}
	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := deleteLastWord(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}
