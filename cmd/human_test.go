package cmd

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestHumanListCmdExists(t *testing.T) {
	cmd := humanListCmd
	assert.NotNil(t, cmd)
	assert.Equal(t, "list", cmd.Name())
	assert.Equal(t, "List all humans", cmd.Short)
}

func TestHumanInfoCmdExists(t *testing.T) {
	cmd := humanInfoCmd
	assert.NotNil(t, cmd)
	assert.Equal(t, "info", cmd.Name())
	assert.Equal(t, "Show details for a human", cmd.Short)
}

func TestHumanCmdExists(t *testing.T) {
	cmd := humanCmd
	assert.NotNil(t, cmd)
	assert.Equal(t, "human", cmd.Name())
	assert.Equal(t, "Manage humans", cmd.Short)
}

func TestTruncate(t *testing.T) {
	tests := []struct {
		input    string
		max      int
		expected string
	}{
		{"short", 10, "short"},
		{"exactly10!", 10, "exactly10!"},
		{"toolongstring", 8, "toolongs…"},
		{"", 5, ""},
	}
	for _, tt := range tests {
		got := truncate(tt.input, tt.max)
		assert.Equal(t, tt.expected, got, "truncate(%q, %d)", tt.input, tt.max)
	}
}
