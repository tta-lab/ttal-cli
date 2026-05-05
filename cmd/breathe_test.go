package cmd

import (
	"strings"
	"testing"

	"github.com/tta-lab/ttal-cli/internal/daemon"
)

func TestRunBreathe_NoTarget_NoFlag_NoEnv(t *testing.T) {
	t.Setenv("TTAL_AGENT_NAME", "")
	t.Setenv("TTAL_SESSION_NAME", "")

	err := runBreathe(breatheCmd, nil)
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "must run from within an agent session") {
		t.Errorf("expected 'must run from within an agent session', got: %v", err)
	}
}

func TestRunBreathe_RejectsPositionalArgs(t *testing.T) {
	t.Setenv("TTAL_AGENT_NAME", "kestrel")

	err := runBreathe(breatheCmd, []string{"# Handoff"})
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "no longer accepts handoff arguments") {
		t.Errorf("expected 'no longer accepts handoff arguments', got: %v", err)
	}
	if !strings.Contains(err.Error(), "skill get breathe") {
		t.Errorf("expected error mentioning 'skill get breathe', got: %v", err)
	}
}

func TestRunBreathe_AgentFlagWins(t *testing.T) {
	t.Setenv("TTAL_AGENT_NAME", "bar")
	t.Setenv("TTAL_SESSION_NAME", "")
	breatheCmd.Flags().Set("agent", "")

	var captured daemon.BreatheRequest
	saved := breatheFn
	breatheFn = func(req daemon.BreatheRequest) error {
		captured = req
		return nil
	}
	defer func() { breatheFn = saved }()

	breatheCmd.Flags().Set("agent", "foo")

	err := runBreathe(breatheCmd, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if captured.Agent != "foo" {
		t.Errorf("expected agent=foo, got agent=%s", captured.Agent)
	}
}

func TestRunBreathe_EnvFallback(t *testing.T) {
	t.Setenv("TTAL_AGENT_NAME", "bar")
	t.Setenv("TTAL_SESSION_NAME", "")
	breatheCmd.Flags().Set("agent", "")

	var captured daemon.BreatheRequest
	saved := breatheFn
	breatheFn = func(req daemon.BreatheRequest) error {
		captured = req
		return nil
	}
	defer func() { breatheFn = saved }()

	err := runBreathe(breatheCmd, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if captured.Agent != "bar" {
		t.Errorf("expected agent=bar, got agent=%s", captured.Agent)
	}
}
