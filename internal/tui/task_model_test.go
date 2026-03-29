package tui

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/tta-lab/ttal-cli/internal/taskwarrior"
)

func TestAge(t *testing.T) {
	tests := []struct {
		name     string
		entry    string
		expected string
	}{
		{"empty entry", "", ""},
		{"invalid entry", "bad-date", "?"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			task := taskwarrior.Task{Entry: tt.entry}
			assert.Equal(t, tt.expected, task.Age())
		})
	}

	// Use -5h30m to avoid truncation boundary flakiness
	recent := time.Now().Add(-5*time.Hour - 30*time.Minute).UTC().Format("20060102T150405Z")
	task := taskwarrior.Task{Entry: recent}
	assert.Equal(t, "6h", task.Age())
}
