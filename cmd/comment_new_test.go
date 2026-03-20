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

func TestResolveCurrentTask_NoEnv_ReturnsError(t *testing.T) {
	_ = os.Unsetenv("TTAL_JOB_ID")
	_ = os.Unsetenv("TTAL_AGENT_NAME")
	_, err := resolveCurrentTask()
	if err == nil {
		t.Error("expected error when no env vars set")
	}
}
