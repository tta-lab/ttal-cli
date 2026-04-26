package launchcmd

import (
	"strings"
	"testing"
)

func TestBuildCCDirectCommand_WithTrigger(t *testing.T) {
	got := BuildCCDirectCommand("/usr/bin/ttal", "coder", ContextTrigger)
	if !strings.Contains(got, "--agent coder") {
		t.Errorf("missing --agent coder: %q", got)
	}
	if !strings.Contains(got, "ttal context") {
		t.Errorf("missing ttal context trigger: %q", got)
	}
	if strings.Contains(got, "--resume") {
		t.Errorf("should not contain --resume: %q", got)
	}
}

func TestBuildCCDirectCommand_NoTrigger(t *testing.T) {
	got := BuildCCDirectCommand("/usr/bin/ttal", "pr-review-lead", "")
	if !strings.Contains(got, "--agent pr-review-lead") {
		t.Errorf("missing --agent: %q", got)
	}
	if strings.Contains(got, "-- '") {
		t.Errorf("should not have trigger when empty: %q", got)
	}
}

func TestBuildCCDirectCommand_ApostropheEscaping(t *testing.T) {
	got := BuildCCDirectCommand("/usr/bin/ttal", "coder", "it's a test")
	if !strings.Contains(got, "it'\\''s a test") {
		t.Errorf("apostrophe not escaped correctly: %q", got)
	}
}

func TestBuildLenosCommand_Basic(t *testing.T) {
	got := BuildLenosCommand("/usr/bin/ttal", "coder", ContextTrigger)
	if !strings.Contains(got, "--agent coder") {
		t.Errorf("missing --agent coder: %q", got)
	}
	if !strings.Contains(got, "ttal context") {
		t.Errorf("missing ttal context trigger: %q", got)
	}
}

func TestBuildLenosCommand_ApostropheEscaping(t *testing.T) {
	got := BuildLenosCommand("/usr/bin/ttal", "coder", "it's a test")
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
