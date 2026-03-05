package launchcmd

import (
	"testing"

	"github.com/tta-lab/ttal-cli/internal/runtime"
)

func TestBuildGatekeeperCommand(t *testing.T) {
	tests := []struct {
		name string
		rt   runtime.Runtime
		want string
		err  bool
	}{
		{
			name: "claude-code",
			rt:   runtime.ClaudeCode,
			want: "ttal worker gatekeeper --task-file /tmp/task.txt -- claude --model opus --dangerously-skip-permissions --",
		},
		{
			name: "opencode",
			rt:   runtime.OpenCode,
			want: "ttal worker gatekeeper --task-file /tmp/task.txt -- opencode --prompt",
		},
		{
			name: "codex",
			rt:   runtime.Codex,
			want: "ttal worker gatekeeper --task-file /tmp/task.txt -- codex --yolo --prompt",
		},
		{
			name: "non-worker runtime returns error",
			rt:   runtime.OpenClaw,
			err:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := BuildGatekeeperCommand("ttal", "/tmp/task.txt", tt.rt)
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
