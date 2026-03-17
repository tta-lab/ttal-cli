package cmd

import (
	"strings"
	"testing"
)

func TestRunBreathe_NoAgentName(t *testing.T) {
	t.Setenv("TTAL_AGENT_NAME", "")

	err := runBreathe(breatheCmd, []string{"# Handoff"})
	if err == nil {
		t.Fatal("expected error when TTAL_AGENT_NAME is not set")
	}
	if !strings.Contains(err.Error(), "TTAL_AGENT_NAME not set") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestRunBreathe_EmptyHandoff(t *testing.T) {
	t.Setenv("TTAL_AGENT_NAME", "kestrel")

	err := runBreathe(breatheCmd, []string{""})
	if err == nil {
		t.Fatal("expected error for empty handoff")
	}
	if !strings.Contains(err.Error(), "handoff prompt is required") {
		t.Errorf("unexpected error: %v", err)
	}
}
