package launchcmd

import (
	"strings"
	"testing"
)

func TestBuildCCDirectCommand_WithTrigger(t *testing.T) {
	got := BuildCCDirectCommand("/usr/bin/ttal", "coder", "Begin implementation.", "")
	if !strings.Contains(got, "--agent coder") {
		t.Errorf("missing --agent coder: %q", got)
	}
	if !strings.Contains(got, "-- 'Begin implementation.'") {
		t.Errorf("missing trigger: %q", got)
	}
	if strings.Contains(got, "--resume") {
		t.Errorf("should not contain --resume: %q", got)
	}
	if strings.Contains(got, "--mcp-config") {
		t.Errorf("should not contain --mcp-config when empty: %q", got)
	}
}

func TestBuildCCDirectCommand_NoTrigger(t *testing.T) {
	got := BuildCCDirectCommand("/usr/bin/ttal", "pr-review-lead", "", "")
	if !strings.Contains(got, "--agent pr-review-lead") {
		t.Errorf("missing --agent: %q", got)
	}
	if strings.Contains(got, "-- '") {
		t.Errorf("should not have trigger when empty: %q", got)
	}
}

func TestBuildCCDirectCommand_ApostropheEscaping(t *testing.T) {
	got := BuildCCDirectCommand("/usr/bin/ttal", "coder", "it's a test", "")
	if !strings.Contains(got, "it'\\''s a test") {
		t.Errorf("apostrophe not escaped correctly: %q", got)
	}
}

func TestBuildCCDirectCommand_MCPConfig(t *testing.T) {
	// Use a short token for line-length compliance.
	mcpJSON := `{"mcpServers":{"temenos":{"type":"http","url":"http://127.0.0.1:9783",` +
		`"headers":{"X-Session-Token":"tok"}}}}`
	got := BuildCCDirectCommand("/usr/bin/ttal", "coder", "Begin.", mcpJSON)
	if !strings.Contains(got, "--mcp-config '"+mcpJSON+"'") {
		t.Errorf("missing --mcp-config: %q", got)
	}
	if !strings.Contains(got, "--agent coder") {
		t.Errorf("missing --agent coder: %q", got)
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
