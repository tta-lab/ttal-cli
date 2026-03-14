package cmd

import (
	"testing"

	"github.com/tta-lab/ttal-cli/internal/gitprovider"
)

func TestPRCICommandExists(t *testing.T) {
	cmd, _, err := rootCmd.Find([]string{"pr", "ci"})
	if err != nil {
		t.Fatalf("pr ci command not found: %v", err)
	}
	if cmd.Name() != "ci" {
		t.Errorf("expected command name 'ci', got %q", cmd.Name())
	}
}

func TestHasCIFailures(t *testing.T) {
	tests := []struct {
		state    string
		expected bool
	}{
		{gitprovider.StateFailure, true},
		{gitprovider.StateError, true},
		{gitprovider.StateSuccess, false},
		{gitprovider.StatePending, false},
		{"unknown", false},
	}
	for _, tt := range tests {
		cs := &gitprovider.CombinedStatus{State: tt.state}
		if got := hasCIFailures(cs); got != tt.expected {
			t.Errorf("hasCIFailures(%q) = %v, want %v", tt.state, got, tt.expected)
		}
	}
}

func TestFormatCIState(t *testing.T) {
	tests := []struct {
		state    string
		expected string
	}{
		{gitprovider.StateSuccess, "passed"},
		{gitprovider.StateFailure, "failed"},
		{gitprovider.StateError, "error"},
		{gitprovider.StatePending, "pending"},
		{"unknown-state", "unknown-state"},
	}
	for _, tt := range tests {
		if got := formatCIState(tt.state); got != tt.expected {
			t.Errorf("formatCIState(%q) = %q, want %q", tt.state, got, tt.expected)
		}
	}
}

func TestCIStateIcon(t *testing.T) {
	tests := []struct {
		state    string
		expected string
	}{
		{gitprovider.StateSuccess, ciIconSuccess},
		{gitprovider.StateFailure, "✗"},
		{gitprovider.StateError, "✗"},
		{gitprovider.StatePending, "·"},
		{"unknown", "?"},
	}
	for _, tt := range tests {
		if got := ciStateIcon(tt.state); got != tt.expected {
			t.Errorf("ciStateIcon(%q) = %q, want %q", tt.state, got, tt.expected)
		}
	}
}
