package worker

import (
	"strings"
	"testing"

	"github.com/tta-lab/ttal-cli/internal/launchcmd"
	"github.com/tta-lab/ttal-cli/internal/runtime"
	"github.com/tta-lab/ttal-cli/internal/taskwarrior"
)

func TestResolveRuntime(t *testing.T) {
	tests := []struct {
		name     string
		configRT runtime.Runtime
		want     runtime.Runtime
	}{
		{
			name:     "explicit claude-code",
			configRT: runtime.ClaudeCode,
			want:     runtime.ClaudeCode,
		},
		{
			name:     "explicit lenos",
			configRT: runtime.Lenos,
			want:     runtime.Lenos,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := resolveRuntime(tt.configRT, nil)
			if got != tt.want {
				t.Errorf("resolveRuntime(%q) = %q, want %q", tt.configRT, got, tt.want)
			}
		})
	}
}

func TestLaunchTmuxWorker_SmallModel_Lenos(t *testing.T) {
	origNewSession := tmuxNewSessionFn
	origExec := osExecFnWorker
	defer func() {
		tmuxNewSessionFn = origNewSession
		osExecFnWorker = origExec
	}()

	var capturedShellCmd string
	tmuxNewSessionFn = func(session, window, workDir, shellCmd string) error {
		capturedShellCmd = shellCmd
		return nil
	}

	osExecFnWorker = func() (string, error) {
		return "/usr/bin/ttal", nil
	}

	task := &taskwarrior.Task{
		UUID:        "abcd1234-0000-0000-0000-000000000000",
		Description: "test task",
		Tags:        []string{"feature"},
	}

	spawnCfg := SpawnConfig{
		Name:      "test-hex",
		Project:   "/tmp/test",
		TaskUUID:  task.UUID,
		Worktree:  false,
		Runtime:   runtime.Lenos,
		AgentName: "coder",
	}

	err := launchTmuxWorker(spawnCfg, task, "test-session", "/tmp/test")
	if err != nil {
		t.Fatalf("launchTmuxWorker: %v", err)
	}

	if !strings.Contains(capturedShellCmd, "lenos --agent coder") {
		t.Errorf("expected lenos agent, got: %q", capturedShellCmd)
	}
	if !strings.Contains(capturedShellCmd, "--small-model") {
		t.Errorf("expected --small-model in worker lenos command, got: %q", capturedShellCmd)
	}
	if !strings.Contains(capturedShellCmd, launchcmd.ContextTrigger) {
		t.Errorf("shellCmd does not contain ContextTrigger: %q", capturedShellCmd)
	}
	if strings.Contains(capturedShellCmd, "claude") {
		t.Errorf("should not contain claude: %q", capturedShellCmd)
	}
}
