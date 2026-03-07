package tui

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestFormatAge(t *testing.T) {
	tests := []struct {
		name     string
		d        time.Duration
		expected string
	}{
		{"minutes", 45 * time.Minute, "45m"},
		{"one hour", time.Hour, "1h"},
		{"hours", 5 * time.Hour, "5h"},
		{"one day", 24 * time.Hour, "1d"},
		{"days", 10 * 24 * time.Hour, "10d"},
		{"29 days", 29 * 24 * time.Hour, "29d"},
		{"30 days becomes months", 30 * 24 * time.Hour, "1mo"},
		{"months", 90 * 24 * time.Hour, "3mo"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := formatAge(tt.d)
			assert.Equal(t, tt.expected, result)
		})
	}
}
