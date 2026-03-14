package launchcmd

import (
	"testing"

	"github.com/tta-lab/ttal-cli/internal/runtime"
)

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
