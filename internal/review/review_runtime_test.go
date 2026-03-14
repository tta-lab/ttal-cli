package review

import (
	"fmt"
	"strings"
	"testing"

	"github.com/tta-lab/ttal-cli/internal/runtime"
)

func TestBuildReviewerRuntimeCmd(t *testing.T) {
	ccBase := "ttal worker gatekeeper --task-file /tmp/prompt.txt -- claude"
	codexBase := "ttal worker gatekeeper --task-file /tmp/prompt.txt -- codex"

	tests := []struct {
		name  string
		rt    runtime.Runtime
		model string
		want  string
		err   bool
	}{
		{
			name:  "claude-code uses sonnet by default",
			rt:    runtime.ClaudeCode,
			model: "sonnet",
			want:  ccBase + " --model sonnet --dangerously-skip-permissions --agent pr-review-lead --",
		},
		{
			name:  "claude-code with opus model",
			rt:    runtime.ClaudeCode,
			model: "opus",
			want:  ccBase + " --model opus --dangerously-skip-permissions --agent pr-review-lead --",
		},
		{
			name:  "codex",
			rt:    runtime.Codex,
			model: "",
			want:  codexBase + " --yolo --",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := buildReviewerRuntimeCmd("ttal", "/tmp/prompt.txt", tt.rt, tt.model)
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

func TestBuildReviewerRuntimeCmd_InterpolatesPaths(t *testing.T) {
	ttalBin := "/usr/local/bin/ttal"
	promptFile := "/tmp/review-123.txt"
	got, err := buildReviewerRuntimeCmd(ttalBin, promptFile, runtime.ClaudeCode, "sonnet")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	wantPrefix := fmt.Sprintf("%s worker gatekeeper --task-file %s", ttalBin, promptFile)
	if !strings.HasPrefix(got, wantPrefix) {
		t.Fatalf("command should start with %q, got %q", wantPrefix, got)
	}
}
