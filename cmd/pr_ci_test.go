package cmd

import (
	"testing"

	"github.com/tta-lab/ttal-cli/internal/daemon"
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

func TestHasDaemonCIFailures(t *testing.T) {
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
		resp := daemon.PRCIStatusResponse{State: tt.state}
		if got := hasDaemonCIFailures(resp); got != tt.expected {
			t.Errorf("hasDaemonCIFailures(%q) = %v, want %v", tt.state, got, tt.expected)
		}
	}
}

func TestFormatDaemonCIState(t *testing.T) {
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
		if got := formatDaemonCIState(tt.state); got != tt.expected {
			t.Errorf("formatDaemonCIState(%q) = %q, want %q", tt.state, got, tt.expected)
		}
	}
}

func TestDaemonCIStateIcon(t *testing.T) {
	tests := []struct {
		state    string
		expected string
	}{
		{gitprovider.StateSuccess, "✓"},
		{gitprovider.StateFailure, "✗"},
		{gitprovider.StateError, "✗"},
		{gitprovider.StatePending, "·"},
		{"unknown", "?"},
	}
	for _, tt := range tests {
		if got := daemonCIStateIcon(tt.state); got != tt.expected {
			t.Errorf("daemonCIStateIcon(%q) = %q, want %q", tt.state, got, tt.expected)
		}
	}
}
