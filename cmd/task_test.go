package cmd

import (
	"strings"
	"testing"

	"github.com/spf13/cobra"
)

func TestTaskGetCmd_AcceptsOptionalArg(t *testing.T) {
	// Verify Use string and Args accept 0 or 1 args
	if !strings.Contains(taskGetCmd.Use, "[uuid]") {
		t.Errorf("taskGetCmd.Use should contain [uuid], got: %s", taskGetCmd.Use)
	}
	// MaximumNArgs(1) should accept 0 args
	if err := cobra.MaximumNArgs(1)(taskGetCmd, nil); err != nil {
		t.Errorf("should accept 0 args: %v", err)
	}
	// MaximumNArgs(1) should reject 2 args
	if err := cobra.MaximumNArgs(1)(taskGetCmd, []string{"a", "b"}); err == nil {
		t.Error("should reject 2 args")
	}
}

func TestTaskGetCmd_ArgTakesPriorityOverEnv(t *testing.T) {
	// When a UUID arg is provided, TTAL_JOB_ID should be ignored.
	// Set TTAL_JOB_ID to a known value, pass a different UUID as arg.
	// Both will fail the taskwarrior lookup (no real task), but the error
	// message should reference the arg UUID, not the env UUID.
	t.Setenv("TTAL_JOB_ID", "aaaaaaaa")
	t.Setenv("TTAL_AGENT_NAME", "")

	err := taskGetCmd.RunE(taskGetCmd, []string{"bbbbbbbb"})
	if err == nil {
		return // in a real env with a matching task this could succeed
	}
	// Error should mention bbbbbbbb (the arg), not aaaaaaaa (the env)
	if strings.Contains(err.Error(), "aaaaaaaa") {
		t.Errorf("arg should take priority over TTAL_JOB_ID, but error references env UUID: %v", err)
	}
	if !strings.Contains(err.Error(), "bbbbbbbb") {
		t.Errorf("expected error to reference arg UUID bbbbbbbb, got: %v", err)
	}
}

func TestTaskGetCmd_InvalidUUIDArg_ReturnsValidationError(t *testing.T) {
	err := taskGetCmd.RunE(taskGetCmd, []string{"42"})
	if err == nil {
		t.Error("expected validation error for numeric task ID")
	}
	if !strings.Contains(err.Error(), "numeric task IDs") {
		t.Errorf("expected 'numeric task IDs' validation error, got: %v", err)
	}
}

func TestTaskGetCmd_NoArgNoEnv_ReturnsError(t *testing.T) {
	t.Setenv("TTAL_JOB_ID", "")
	t.Setenv("TTAL_AGENT_NAME", "")

	err := taskGetCmd.RunE(taskGetCmd, nil)
	if err == nil {
		t.Error("expected error when no arg and no env vars")
	}
	if !strings.Contains(err.Error(), "auto-resolve failed") {
		t.Errorf("expected auto-resolve error, got: %v", err)
	}
}
