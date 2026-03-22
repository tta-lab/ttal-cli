package cmd

import (
	"os"
	"testing"
)

func TestCommentCmdExists(t *testing.T) {
	var found bool
	for _, sub := range rootCmd.Commands() {
		if sub.Name() == "comment" {
			found = true
			break
		}
	}
	if !found {
		t.Error("ttal comment command not found")
	}
}

func TestCommentAddSubcmdExists(t *testing.T) {
	var found bool
	for _, sub := range newCommentCmd.Commands() {
		if sub.Name() == "add" {
			found = true
			break
		}
	}
	if !found {
		t.Error("ttal comment add subcommand not found")
	}
}

func TestCommentListSubcmdExists(t *testing.T) {
	var found bool
	for _, sub := range newCommentCmd.Commands() {
		if sub.Name() == "list" {
			found = true
			break
		}
	}
	if !found {
		t.Error("ttal comment list subcommand not found")
	}
}

func TestCommentLgtmSubcmdExists(t *testing.T) {
	var found bool
	for _, sub := range newCommentCmd.Commands() {
		if sub.Name() == "lgtm" {
			found = true
			break
		}
	}
	if !found {
		t.Error("ttal comment lgtm subcommand not found")
	}
}

func TestCommentGetSubcmdExists(t *testing.T) {
	var found bool
	for _, sub := range newCommentCmd.Commands() {
		if sub.Name() == "get" {
			found = true
			break
		}
	}
	if !found {
		t.Error("ttal comment get subcommand not found")
	}
}

func TestResolveCurrentTask_NoEnv_ReturnsError(t *testing.T) {
	_ = os.Unsetenv("TTAL_JOB_ID")
	_ = os.Unsetenv("TTAL_AGENT_NAME")
	_, err := resolveCurrentTask()
	if err == nil {
		t.Error("expected error when no env vars set")
	}
}

func TestIsReviewer_NoConfig_ReturnsFalse(t *testing.T) {
	// With no pipeline config, isReviewer should return false (not panic).
	result := isReviewer("nonexistent-agent")
	if result {
		t.Error("expected isReviewer to return false when pipeline config is unavailable")
	}
}

func TestResolveCurrentTask_WithJobID_AttemptsLookup(t *testing.T) {
	// When TTAL_JOB_ID is set, resolveCurrentTask should attempt the taskwarrior
	// lookup and return an error (since no real task exists in test env) — not the
	// "no env vars" error. This confirms the TTAL_JOB_ID branch is taken.
	t.Setenv("TTAL_JOB_ID", "f9a917aa")
	_ = os.Unsetenv("TTAL_AGENT_NAME")
	_, err := resolveCurrentTask()
	if err == nil {
		// In a real env with a matching task this would succeed — both outcomes are valid.
		return
	}
	// The error should be about the job ID lookup, not "no TTAL_JOB_ID or TTAL_AGENT_NAME set".
	if err.Error() == "no TTAL_JOB_ID or TTAL_AGENT_NAME set — cannot resolve task" {
		t.Errorf("TTAL_JOB_ID branch was not taken: %v", err)
	}
}
