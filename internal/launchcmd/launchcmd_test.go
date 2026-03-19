package launchcmd

import (
	"strings"
	"testing"

	"github.com/tta-lab/ttal-cli/internal/runtime"
)

func TestBuildResumeCommand(t *testing.T) {
	cmd, err := BuildResumeCommand("/usr/bin/ttal", "session-abc", runtime.ClaudeCode, "sonnet", "kestrel", "Review the PR.")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(cmd, "--resume session-abc") {
		t.Errorf("missing --resume: %q", cmd)
	}
	if !strings.Contains(cmd, "--agent kestrel") {
		t.Errorf("missing --agent: %q", cmd)
	}
	if !strings.Contains(cmd, "-- 'Review the PR.'") {
		t.Errorf("missing trigger: %q", cmd)
	}
}

func TestBuildResumeCommandNoTrigger(t *testing.T) {
	cmd, err := BuildResumeCommand("/usr/bin/ttal", "session-abc", runtime.ClaudeCode, "", "", "")
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(cmd, "-- '") {
		t.Errorf("should not have trigger when empty: %q", cmd)
	}
	if strings.Contains(cmd, "--agent") {
		t.Errorf("should not have --agent when empty: %q", cmd)
	}
	if !strings.Contains(cmd, "--model sonnet") {
		t.Errorf("model should default to sonnet: %q", cmd)
	}
}

func TestBuildResumeCommandCodexUnsupported(t *testing.T) {
	_, err := BuildResumeCommand("/usr/bin/ttal", "session-abc", runtime.Codex, "", "", "")
	if err == nil {
		t.Fatal("expected error for Codex runtime")
	}
	if !strings.Contains(err.Error(), "#321") {
		t.Errorf("error should reference tracking issue: %v", err)
	}
}

func TestBuildGatekeeperCommand(t *testing.T) {
	ccBase := "ttal worker gatekeeper --task-file /tmp/task.txt -- claude"
	codexBase := "ttal worker gatekeeper --task-file /tmp/task.txt -- codex"

	tests := []struct {
		name  string
		rt    runtime.Runtime
		model string
		agent string
		want  string
		err   bool
	}{
		{
			name:  "claude-code reviewer with sonnet",
			rt:    runtime.ClaudeCode,
			model: "sonnet",
			agent: "pr-review-lead",
			want:  ccBase + " --model sonnet --dangerously-skip-permissions --agent pr-review-lead --",
		},
		{
			name:  "claude-code reviewer with opus",
			rt:    runtime.ClaudeCode,
			model: "opus",
			agent: "pr-review-lead",
			want:  ccBase + " --model opus --dangerously-skip-permissions --agent pr-review-lead --",
		},
		{
			name:  "claude-code reviewer empty model defaults to sonnet",
			rt:    runtime.ClaudeCode,
			model: "",
			agent: "pr-review-lead",
			want:  ccBase + " --model sonnet --dangerously-skip-permissions --agent pr-review-lead --",
		},
		{
			name:  "claude-code coder (no agent)",
			rt:    runtime.ClaudeCode,
			model: "sonnet",
			agent: "",
			want:  ccBase + " --model sonnet --dangerously-skip-permissions --",
		},
		{
			name:  "codex ignores model and agent",
			rt:    runtime.Codex,
			model: "opus",
			agent: "",
			want:  codexBase + " --yolo --",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := BuildGatekeeperCommand("ttal", "/tmp/task.txt", tt.rt, tt.model, tt.agent)
			if tt.err {
				if err == nil {
					t.Fatalf("expected error, got command: %s", got)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != tt.want {
				t.Fatalf("unexpected command\nwant: %s\n got: %s", tt.want, got)
			}
		})
	}
}
