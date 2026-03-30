package launchcmd

import (
	"strings"
	"testing"

	"github.com/tta-lab/ttal-cli/internal/runtime"
)

func TestBuildResumeCommand(t *testing.T) {
	got, err := BuildResumeCommand(
		"/usr/bin/ttal", "session-abc", runtime.ClaudeCode, "kestrel", "Review the PR.",
	)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(got, "--resume session-abc") {
		t.Errorf("missing --resume: %q", got)
	}
	if !strings.Contains(got, "--agent kestrel") {
		t.Errorf("missing --agent: %q", got)
	}
	if !strings.Contains(got, "-- 'Review the PR.'") {
		t.Errorf("missing trigger: %q", got)
	}
	if strings.Contains(got, "--model") {
		t.Errorf("should not contain --model: %q", got)
	}
}

func TestBuildResumeCommandNoTrigger(t *testing.T) {
	got, err := BuildResumeCommand("/usr/bin/ttal", "session-abc", runtime.ClaudeCode, "", "")
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(got, "-- '") {
		t.Errorf("should not have trigger when empty: %q", got)
	}
	if strings.Contains(got, "--agent") {
		t.Errorf("should not have --agent when empty: %q", got)
	}
	if strings.Contains(got, "--model") {
		t.Errorf("should not have --model: %q", got)
	}
}

func TestBuildResumeCommandCodexUnsupported(t *testing.T) {
	_, err := BuildResumeCommand("/usr/bin/ttal", "session-abc", runtime.Codex, "", "")
	if err == nil {
		t.Fatal("expected error for Codex runtime")
	}
	if !strings.Contains(err.Error(), "#321") {
		t.Errorf("error should reference tracking issue: %v", err)
	}
}

func TestBuildResumeCommandApostropheEscaping(t *testing.T) {
	got, err := BuildResumeCommand(
		"/usr/bin/ttal", "session-abc", runtime.ClaudeCode, "", "it's a test",
	)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(got, "it'\\''s a test") {
		t.Errorf("apostrophe not escaped correctly: %q", got)
	}
}

func TestBuildCodexGatekeeperCommand(t *testing.T) {
	got, err := BuildCodexGatekeeperCommand("ttal", "/tmp/task.txt")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := "ttal worker gatekeeper --task-file /tmp/task.txt -- codex --yolo --"
	if got != want {
		t.Fatalf("unexpected command\nwant: %s\n got: %s", want, got)
	}
}
