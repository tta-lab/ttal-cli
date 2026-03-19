package review

import (
	"strings"
	"testing"

	"github.com/tta-lab/ttal-cli/internal/launchcmd"
	"github.com/tta-lab/ttal-cli/internal/runtime"
)

// TestCodexReviewerCmd verifies the Codex reviewer still uses the legacy task-file path.
func TestCodexReviewerCmd(t *testing.T) {
	got, err := launchcmd.BuildCodexGatekeeperCommand("ttal", "/tmp/prompt.txt")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(got, "codex --yolo --") {
		t.Errorf("Codex command should contain 'codex --yolo --', got: %s", got)
	}
	if !strings.Contains(got, "--task-file /tmp/prompt.txt") {
		t.Errorf("Codex command should contain --task-file, got: %s", got)
	}
}

// TestCCReviewerCmd verifies the CC reviewer uses BuildResumeCommand with --resume and --agent.
func TestCCReviewerCmd(t *testing.T) {
	got, err := launchcmd.BuildResumeCommand(
		"ttal", "session-xyz", runtime.ClaudeCode, "sonnet", "pr-review-lead", "Review the PR.",
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(got, "--resume session-xyz") {
		t.Errorf("CC command should use --resume, got: %s", got)
	}
	if !strings.Contains(got, "--agent pr-review-lead") {
		t.Errorf("CC command should pass --agent pr-review-lead, got: %s", got)
	}
	if !strings.Contains(got, "Review the PR.") {
		t.Errorf("CC command should contain trigger, got: %s", got)
	}
	if strings.Contains(got, "--task-file") {
		t.Errorf("CC command should NOT use --task-file, got: %s", got)
	}
}
