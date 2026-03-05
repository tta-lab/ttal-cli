package launchcmd

import (
	"testing"

	"github.com/tta-lab/ttal-cli/internal/runtime"
)

func TestBuildGatekeeperCommand(t *testing.T) {
	tests := []struct {
		name string
		rt   runtime.Runtime
		opts Options
		want string
	}{
		{
			name: "claude-code with defaults",
			rt:   runtime.ClaudeCode,
			opts: Options{},
			want: "ttal worker gatekeeper --task-file /tmp/task.txt -- claude --model opus --",
		},
		{
			name: "claude-code with yolo and sonnet",
			rt:   runtime.ClaudeCode,
			opts: Options{ClaudeModel: "sonnet", ClaudeYolo: true},
			want: "ttal worker gatekeeper --task-file /tmp/task.txt -- claude --model sonnet --dangerously-skip-permissions --",
		},
		{
			name: "opencode",
			rt:   runtime.OpenCode,
			opts: Options{},
			want: "ttal worker gatekeeper --task-file /tmp/task.txt -- opencode --prompt",
		},
		{
			name: "codex no yolo",
			rt:   runtime.Codex,
			opts: Options{},
			want: "ttal worker gatekeeper --task-file /tmp/task.txt -- codex --prompt",
		},
		{
			name: "codex with yolo",
			rt:   runtime.Codex,
			opts: Options{CodexYolo: true},
			want: "ttal worker gatekeeper --task-file /tmp/task.txt -- codex --yolo --prompt",
		},
		{
			name: "non-worker runtime falls back to claude",
			rt:   runtime.OpenClaw,
			opts: Options{ClaudeYolo: true},
			want: "ttal worker gatekeeper --task-file /tmp/task.txt -- claude --model opus --dangerously-skip-permissions --",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := BuildGatekeeperCommand("ttal", "/tmp/task.txt", tt.rt, tt.opts)
			if got != tt.want {
				t.Fatalf("unexpected command\nwant: %s\n got: %s", tt.want, got)
			}
		})
	}
}
