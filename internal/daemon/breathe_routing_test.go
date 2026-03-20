package daemon

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/tta-lab/ttal-cli/internal/config"
	"github.com/tta-lab/ttal-cli/internal/route"
)

func TestHandleBreatheValidation(t *testing.T) {
	shellCfg := &config.Config{}

	tests := []struct {
		name    string
		req     BreatheRequest
		wantErr string
	}{
		{
			name:    "missing agent",
			req:     BreatheRequest{Agent: "", Handoff: "# Handoff"},
			wantErr: "missing agent name",
		},
		{
			name:    "empty handoff",
			req:     BreatheRequest{Agent: "kestrel", Handoff: ""},
			wantErr: "empty handoff prompt",
		},
		{
			name:    "session not found (valid agent + handoff but no tmux)",
			req:     BreatheRequest{Agent: "nonexistent-test-agent-xyz", Handoff: "# Handoff\n\nNext steps: continue"},
			wantErr: "cannot resolve agent workspace path",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resp := handleBreathe(shellCfg, tt.req)
			if resp.OK {
				t.Fatalf("expected OK=false, got OK=true")
			}
			if tt.wantErr != "" && resp.Error == "" {
				t.Errorf("expected error containing %q, got empty", tt.wantErr)
			}
		})
	}
}

func TestHandleBreatheTeamDefault(t *testing.T) {
	shellCfg := &config.Config{}

	// team="" should default without panicking — it will fail at CWD fallback
	// (shellCfg has no resolved team path, so AgentPath returns "").
	resp := handleBreathe(shellCfg, BreatheRequest{
		Team:    "",
		Agent:   "nonexistent-test-agent-xyz",
		Handoff: "# Handoff",
	})
	// Should fail at CWD fallback, not at team validation
	if resp.OK {
		t.Fatalf("expected OK=false (no tmux session), got OK=true")
	}
}

// TestBuildCCRestartCmd verifies that --agent flag is present and correctly interpolated.
func TestBuildCCRestartCmd(t *testing.T) {
	cmd := buildCCRestartCmd("session-abc", "sonnet", "kestrel", "")

	if !strings.Contains(cmd, "--resume session-abc") {
		t.Errorf("missing --resume flag: %q", cmd)
	}
	if !strings.Contains(cmd, "--model sonnet") {
		t.Errorf("missing --model flag: %q", cmd)
	}
	if !strings.Contains(cmd, "--agent kestrel") {
		t.Errorf("missing --agent flag: %q", cmd)
	}
	if !strings.Contains(cmd, "--dangerously-skip-permissions") {
		t.Errorf("missing --dangerously-skip-permissions flag: %q", cmd)
	}
	// Empty trigger should produce no -- separator
	if strings.Contains(cmd, "-- ") {
		t.Errorf("empty trigger should not produce -- separator: %q", cmd)
	}
}

func TestBuildCCRestartCmdWithTrigger(t *testing.T) {
	cmd := buildCCRestartCmd("session-123", "sonnet", "inke", "New task: design auth. Run: ttal task get abc12345")
	if !strings.Contains(cmd, "-- 'New task:") {
		t.Errorf("missing trigger with -- separator: %q", cmd)
	}
}

func TestBuildCCRestartCmdEmptyTrigger(t *testing.T) {
	cmd := buildCCRestartCmd("session-123", "sonnet", "inke", "")
	if strings.Contains(cmd, "-- ") {
		t.Errorf("empty trigger should not produce -- separator: %q", cmd)
	}
}

func TestBuildCCRestartCmdApostropheEscaping(t *testing.T) {
	cmd := buildCCRestartCmd("session-abc", "sonnet", "kestrel", "it's a test")
	if !strings.Contains(cmd, "it'\\''s a test") {
		t.Errorf("apostrophe not escaped correctly: %q", cmd)
	}
}

// TestHandleSendSystemRouting verifies that From=="system" routes to handleSystemToAgent
// and not to handleAgentToAgent (which would add an [agent from:] prefix).
func TestHandleSendSystemRouting(t *testing.T) {
	// handleSend with From="system" and a known agent should return an error about
	// the agent not being found (no daemon config in test) — NOT a "send request missing"
	// error, and NOT fall through to handleAgentToAgent logic.
	//
	// We verify the routing by checking the error message: if it routes to
	// handleSystemToAgent, we get "unknown agent: <name>"; if it falls through to
	// handleAgentToAgent it would also resolve the *From* agent and return
	// "unknown agent: system" (failing on sender lookup, not recipient).
	mcfg := &config.DaemonConfig{}
	req := SendRequest{From: "system", To: "athena", Message: "/breathe"}
	err := handleSend(mcfg, nil, nil, nil, req)
	if err == nil {
		t.Fatal("expected error for unknown agent, got nil")
	}
	// handleSystemToAgent resolves the To agent — error must reference the recipient.
	if !strings.Contains(err.Error(), "athena") {
		t.Errorf("expected error about recipient agent 'athena', got: %v", err)
	}
	// Must NOT reference "system" as an unknown agent (which handleAgentToAgent would do
	// when it tries to resolve the From agent first).
	if strings.Contains(err.Error(), "unknown agent: system") {
		t.Errorf("routed to handleAgentToAgent instead of handleSystemToAgent: %v", err)
	}
}

const composeHandoffBase = "# Base Handoff\n\nContext here."

func TestComposeHandoffNoFile(t *testing.T) {
	handoff, trigger, err := composeHandoff("test-composehandoff-no-route-xyz", composeHandoffBase)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if handoff != composeHandoffBase {
		t.Errorf("expected base handoff unchanged, got %q", handoff)
	}
	if trigger != "" {
		t.Errorf("expected empty trigger, got %q", trigger)
	}
}

func TestComposeHandoffRolePromptOnly(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)
	if err := os.MkdirAll(filepath.Join(tmp, ".ttal"), 0o755); err != nil {
		t.Fatal(err)
	}
	agent := "test-composehandoff-roleprompt-xyz"
	if err := route.Stage(agent, route.Request{
		TaskUUID:   "task-abc",
		RolePrompt: "Build the auth module.",
		Trigger:    "auth task ready",
	}); err != nil {
		t.Fatalf("stage failed: %v", err)
	}
	handoff, trigger, err := composeHandoff(agent, composeHandoffBase)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(handoff, "## New Task Assignment") {
		t.Errorf("expected section header in handoff: %q", handoff)
	}
	if !strings.Contains(handoff, "Build the auth module.") {
		t.Errorf("expected role prompt in handoff: %q", handoff)
	}
	if trigger != "auth task ready" {
		t.Errorf("expected trigger %q, got %q", "auth task ready", trigger)
	}
}

func TestComposeHandoffMessageOnly(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)
	if err := os.MkdirAll(filepath.Join(tmp, ".ttal"), 0o755); err != nil {
		t.Fatal(err)
	}
	agent := "test-composehandoff-message-xyz"
	if err := route.Stage(agent, route.Request{
		TaskUUID: "task-def",
		Message:  "Extra context for you.",
		Trigger:  "msg trigger",
	}); err != nil {
		t.Fatalf("stage failed: %v", err)
	}
	handoff, trigger, err := composeHandoff(agent, composeHandoffBase)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if strings.Contains(handoff, "## New Task Assignment") {
		t.Errorf("should not have section header when no role prompt: %q", handoff)
	}
	if !strings.Contains(handoff, "Extra context for you.") {
		t.Errorf("expected message in handoff: %q", handoff)
	}
	if trigger != "msg trigger" {
		t.Errorf("expected trigger %q, got %q", "msg trigger", trigger)
	}
}

func TestComposeHandoffBoth(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)
	if err := os.MkdirAll(filepath.Join(tmp, ".ttal"), 0o755); err != nil {
		t.Fatal(err)
	}
	agent := "test-composehandoff-both-xyz"
	if err := route.Stage(agent, route.Request{
		TaskUUID:   "task-ghi",
		RolePrompt: "Design the API.",
		Message:    "See ticket #42.",
		Trigger:    "design task",
	}); err != nil {
		t.Fatalf("stage failed: %v", err)
	}
	handoff, trigger, err := composeHandoff(agent, composeHandoffBase)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(handoff, "## New Task Assignment") {
		t.Errorf("expected section header: %q", handoff)
	}
	if !strings.Contains(handoff, "Design the API.") {
		t.Errorf("expected role prompt: %q", handoff)
	}
	if !strings.Contains(handoff, "See ticket #42.") {
		t.Errorf("expected message: %q", handoff)
	}
	if trigger != "design task" {
		t.Errorf("expected trigger %q, got %q", "design task", trigger)
	}
}

// TestBuildCCRestartCmdAgentInterpolation verifies agent name is not swapped with session/model.
func TestBuildCCRestartCmdAgentInterpolation(t *testing.T) {
	cmd := buildCCRestartCmd("my-session", "opus", "athena", "")
	if !strings.Contains(cmd, "--agent athena") {
		t.Errorf("agent name not correctly interpolated, got: %q", cmd)
	}
	// Ensure model is not placed in the agent slot
	if strings.Contains(cmd, "--agent opus") {
		t.Errorf("model leaked into --agent slot: %q", cmd)
	}
}

// writeFakeDiary writes a fake diary shell script to tmpDir.
// readOutput is written to a side file so the script can cat it safely —
// avoids shell quoting issues with arbitrary content.
//
// Modes:
//   - "ok"          — append succeeds, read returns readOutput
//   - "fail-append" — append exits non-zero with error on stderr
//   - "empty-read"  — append succeeds, read exits 0 with no output
//   - "fail-read"   — append succeeds, read exits non-zero with error on stderr
func writeFakeDiary(t *testing.T, tmpDir, mode, readOutput string) {
	t.Helper()
	outputFile := filepath.Join(tmpDir, "diary-read-output")
	if err := os.WriteFile(outputFile, []byte(readOutput), 0o644); err != nil {
		t.Fatalf("write diary read output file: %v", err)
	}

	var script string
	switch mode {
	case "ok":
		script = "#!/bin/sh\n" +
			"if [ \"$2\" = \"append\" ]; then cat > /dev/null; exit 0; fi\n" +
			"if [ \"$2\" = \"read\" ]; then cat '" + outputFile + "'; exit 0; fi\n" +
			"exit 1\n"
	case "fail-append":
		script = "#!/bin/sh\necho 'text required for append command' >&2; exit 1\n"
	case "empty-read":
		script = "#!/bin/sh\n" +
			"if [ \"$2\" = \"append\" ]; then cat > /dev/null; exit 0; fi\n" +
			"if [ \"$2\" = \"read\" ]; then exit 0; fi\n" +
			"exit 1\n"
	case "fail-read":
		script = "#!/bin/sh\n" +
			"if [ \"$2\" = \"append\" ]; then cat > /dev/null; exit 0; fi\n" +
			"if [ \"$2\" = \"read\" ]; then echo 'diary read error' >&2; exit 1; fi\n" +
			"exit 1\n"
	}
	diaryPath := filepath.Join(tmpDir, "diary")
	if err := os.WriteFile(diaryPath, []byte(script), 0o755); err != nil {
		t.Fatalf("write fake diary: %v", err)
	}
}

func TestDiaryAppendHandoff(t *testing.T) {
	const handoff = "# Handoff\n\nDid some work."

	t.Run("diary not on PATH is a no-op", func(t *testing.T) {
		t.Setenv("PATH", "/nonexistent-path-xyz")
		// Must not panic — no return value to assert.
		diaryAppendHandoff("kestrel", handoff)
	})

	t.Run("diary append failure does not panic", func(t *testing.T) {
		tmp := t.TempDir()
		writeFakeDiary(t, tmp, "fail-append", "")
		t.Setenv("PATH", tmp+":"+os.Getenv("PATH"))
		diaryAppendHandoff("kestrel", handoff)
	})
}

func TestDiaryReadToday(t *testing.T) {
	const original = "# Handoff\n\nDid some work."

	t.Run("diary not on PATH returns original handoff", func(t *testing.T) {
		t.Setenv("PATH", "/nonexistent-path-xyz")
		got := diaryReadToday("kestrel", original)
		if got != original {
			t.Errorf("expected original handoff, got %q", got)
		}
	})

	t.Run("diary read fails (non-zero exit) returns original handoff", func(t *testing.T) {
		tmp := t.TempDir()
		writeFakeDiary(t, tmp, "fail-read", "")
		t.Setenv("PATH", tmp+":"+os.Getenv("PATH"))
		got := diaryReadToday("kestrel", original)
		if got != original {
			t.Errorf("expected original handoff on read failure, got %q", got)
		}
	})

	t.Run("diary read returns empty falls back to original handoff", func(t *testing.T) {
		tmp := t.TempDir()
		writeFakeDiary(t, tmp, "empty-read", "")
		t.Setenv("PATH", tmp+":"+os.Getenv("PATH"))
		got := diaryReadToday("kestrel", original)
		if got != original {
			t.Errorf("expected original handoff on empty read, got %q", got)
		}
	})

	t.Run("diary available returns enriched handoff from read", func(t *testing.T) {
		tmp := t.TempDir()
		enriched := "# Today\nHandoff + reflection from it's a complex day"
		writeFakeDiary(t, tmp, "ok", enriched)
		t.Setenv("PATH", tmp+":"+os.Getenv("PATH"))
		got := diaryReadToday("kestrel", original)
		if got != enriched {
			t.Errorf("expected enriched handoff %q, got %q", enriched, got)
		}
	})
}
