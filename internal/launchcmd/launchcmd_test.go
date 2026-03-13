package launchcmd

import (
	"testing"

	"github.com/tta-lab/ttal-cli/internal/runtime"
)

func TestBuildGatekeeperCommand(t *testing.T) {
	ccBase := "ttal worker gatekeeper --task-file /tmp/task.txt -- claude"
	ocBase := "ttal worker gatekeeper --task-file /tmp/task.txt -- opencode"
	codexBase := "ttal worker gatekeeper --task-file /tmp/task.txt -- codex"

	tests := []struct {
		name  string
		rt    runtime.Runtime
		model string
		want  string
		err   bool
	}{
		{
			name:  "claude-code with sonnet",
			rt:    runtime.ClaudeCode,
			model: "sonnet",
			want:  ccBase + " --model sonnet --dangerously-skip-permissions --agent pr-review-lead --",
		},
		{
			name:  "claude-code with opus",
			rt:    runtime.ClaudeCode,
			model: "opus",
			want:  ccBase + " --model opus --dangerously-skip-permissions --agent pr-review-lead --",
		},
		{
			name:  "claude-code empty model defaults to sonnet",
			rt:    runtime.ClaudeCode,
			model: "",
			want:  ccBase + " --model sonnet --dangerously-skip-permissions --agent pr-review-lead --",
		},
		{
			name:  "opencode ignores model",
			rt:    runtime.OpenCode,
			model: "opus",
			want:  ocBase + " --prompt --agent pr-review-lead",
		},
		{
			name:  "codex ignores model",
			rt:    runtime.Codex,
			model: "opus",
			want:  codexBase + " --yolo --prompt",
		},
		{
			name: "non-worker runtime returns error",
			rt:   runtime.OpenClaw,
			err:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := BuildGatekeeperCommand("ttal", "/tmp/task.txt", tt.rt, tt.model)
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
