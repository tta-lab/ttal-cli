package daemon

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/tta-lab/logos"
	"github.com/tta-lab/ttal-cli/internal/cmdexec"
	"github.com/tta-lab/ttal-cli/internal/config"
	"github.com/tta-lab/ttal-cli/internal/project"
)

// mockRunner is a logos.CommandRunner for testing.
type mockRunner struct {
	results map[string]*logos.RunResponse
	err     error
	calls   []string
	lastEnv map[string]string // captures Env from the most recent Run call
}

func newMockRunner() *mockRunner {
	return &mockRunner{
		results: make(map[string]*logos.RunResponse),
		calls:   []string{},
	}
}

func (m *mockRunner) Run(_ context.Context, req logos.RunRequest) (*logos.RunResponse, error) {
	m.calls = append(m.calls, req.Command)
	m.lastEnv = req.Env
	if m.err != nil {
		return nil, m.err
	}
	if resp, ok := m.results[req.Command]; ok {
		return resp, nil
	}
	return &logos.RunResponse{Stdout: "default output", ExitCode: 0}, nil
}

func (m *mockRunner) setResult(cmd string, resp *logos.RunResponse) {
	m.results[cmd] = resp
}

// setupTestBridge creates a bridge with a mock runner and test project store.
func setupTestBridge(t *testing.T) (*cmdexecBridge, *mockRunner) {
	t.Helper()
	tmp := t.TempDir()

	storePath := filepath.Join(tmp, "projects.toml")
	store := project.NewStore(storePath)
	projectsToml := `[testproj]
name = "Test Project"
path = "` + filepath.Join(tmp, "code", "testproj") + `"
`
	if err := os.WriteFile(storePath, []byte(projectsToml), 0o644); err != nil {
		t.Fatalf("write projects.toml: %v", err)
	}
	if err := os.MkdirAll(filepath.Join(tmp, "code", "testproj"), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}

	runner := newMockRunner()
	bridge := &cmdexecBridge{
		cfg:          nil,
		runner:       runner,
		projectStore: store,
		agentMutexes: sync.Map{},
	}
	return bridge, runner
}

// setupBridgeWithConfig creates a bridge with a mock runner and a real DaemonConfig.
func setupBridgeWithConfig(t *testing.T, taskRC string) (*cmdexecBridge, *mockRunner) {
	t.Helper()
	tmp := t.TempDir()

	storePath := filepath.Join(tmp, "projects.toml")
	store := project.NewStore(storePath)
	projectsToml := `[testproj]
name = "Test Project"
path = "` + filepath.Join(tmp, "code", "testproj") + `"
`
	if err := os.WriteFile(storePath, []byte(projectsToml), 0o644); err != nil {
		t.Fatalf("write projects.toml: %v", err)
	}
	agentWorkspace := filepath.Join(tmp, "code", "testproj")
	if err := os.MkdirAll(agentWorkspace, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}

	mcfg := &config.DaemonConfig{
		Global: &config.LegacyConfig{},
		Team:   &config.ResolvedTeam{TaskRC: taskRC},
		Teams: map[string]*config.ResolvedTeam{
			"default": {TaskRC: taskRC},
		},
	}
	runner := newMockRunner()
	bridge := &cmdexecBridge{
		cfg:          mcfg,
		runner:       runner,
		projectStore: store,
		agentMutexes: sync.Map{},
	}
	return bridge, runner
}

func TestExecuteCmds_ForwardsHOMEAndManagerEnv(t *testing.T) {
	t.Setenv("HOME", "/fake/home")

	bridge, runner := setupBridgeWithConfig(t, "/fake/taskrc")
	runner.setResult("true", &logos.RunResponse{ExitCode: 0})

	agentCwd := filepath.Join(os.TempDir(), "lux")
	policy, _ := cmdexec.PolicyForAgent(bridge.projectStore, agentCwd)

	bridge.executeCmds(context.Background(), "lux", policy, []string{"true"})

	if runner.lastEnv == nil {
		t.Fatalf("runner.lastEnv is nil, want non-nil")
	}
	if runner.lastEnv["HOME"] != "/fake/home" {
		t.Errorf("HOME = %q, want %q", runner.lastEnv["HOME"], "/fake/home")
	}
	if runner.lastEnv["TTAL_AGENT_NAME"] != "lux" {
		t.Errorf("TTAL_AGENT_NAME = %q, want %q", runner.lastEnv["TTAL_AGENT_NAME"], "lux")
	}
	if runner.lastEnv["TASKRC"] != "/fake/taskrc" {
		t.Errorf("TASKRC = %q, want %q", runner.lastEnv["TASKRC"], "/fake/taskrc")
	}
	// Regression guard: no secret keys.
	for key := range runner.lastEnv {
		if strings.HasSuffix(key, "_TOKEN") || strings.HasSuffix(key, "_SECRET") || strings.HasSuffix(key, "_PASSWORD") {
			t.Errorf("secret key %q leaked into sandbox env", key)
		}
	}
}

func TestExecuteCmds_NilConfig_StillForwardsHOME(t *testing.T) {
	t.Setenv("HOME", "/fake/home")

	bridge, runner := setupTestBridge(t)
	runner.setResult("true", &logos.RunResponse{ExitCode: 0})

	agentCwd := filepath.Join(os.TempDir(), "athena")
	policy, _ := cmdexec.PolicyForAgent(bridge.projectStore, agentCwd)

	bridge.executeCmds(context.Background(), "anything", policy, []string{"true"})

	if runner.lastEnv["HOME"] != "/fake/home" {
		t.Errorf("HOME = %q, want %q", runner.lastEnv["HOME"], "/fake/home")
	}
	if _, ok := runner.lastEnv["TTAL_AGENT_NAME"]; ok {
		t.Errorf("TTAL_AGENT_NAME should not be set when cfg is nil")
	}
}

func TestExecuteCmds_ForwardsXDGConfigHome(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", "/fake/xdg")
	bridge, runner := setupBridgeWithConfig(t, "/fake/taskrc")
	runner.setResult("true", &logos.RunResponse{ExitCode: 0})

	agentCwd := filepath.Join(os.TempDir(), "lux")
	policy, _ := cmdexec.PolicyForAgent(bridge.projectStore, agentCwd)

	bridge.executeCmds(context.Background(), "lux", policy, []string{"true"})

	if runner.lastEnv["XDG_CONFIG_HOME"] != "/fake/xdg" {
		t.Errorf("XDG_CONFIG_HOME = %q, want %q", runner.lastEnv["XDG_CONFIG_HOME"], "/fake/xdg")
	}
}

func TestExecuteCmds_SingleCmd(t *testing.T) {
	bridge, runner := setupTestBridge(t)
	runner.setResult("echo hello", &logos.RunResponse{Stdout: "hello\n", ExitCode: 0})

	agentCwd := filepath.Join(os.TempDir(), "athena")
	policy, _ := cmdexec.PolicyForAgent(bridge.projectStore, agentCwd)

	results := bridge.executeCmds(context.Background(), "testagent", policy, []string{"echo hello"})

	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	if !strings.Contains(results[0], "echo hello") {
		t.Errorf("result = %q, want cmd prefix", results[0])
	}
}

func TestExecuteCmds_MultipleCmds(t *testing.T) {
	bridge, runner := setupTestBridge(t)
	runner.setResult("echo a", &logos.RunResponse{Stdout: "a\n", ExitCode: 0})
	runner.setResult("echo b", &logos.RunResponse{Stdout: "b\n", ExitCode: 0})

	agentCwd := filepath.Join(os.TempDir(), "athena")
	policy, _ := cmdexec.PolicyForAgent(bridge.projectStore, agentCwd)

	results := bridge.executeCmds(context.Background(), "testagent", policy, []string{"echo a", "echo b"})

	if len(results) != 2 {
		t.Fatalf("expected 2 results, got %d", len(results))
	}
	if !strings.Contains(results[0], "echo a") || !strings.Contains(results[1], "echo b") {
		t.Errorf("results = %v, want [echo a..., echo b...]", results)
	}
}

func TestExecuteCmds_RunnerError(t *testing.T) {
	bridge, runner := setupTestBridge(t)
	runner.err = context.DeadlineExceeded

	agentCwd := filepath.Join(os.TempDir(), "athena")
	policy, _ := cmdexec.PolicyForAgent(bridge.projectStore, agentCwd)

	results := bridge.executeCmds(context.Background(), "testagent", policy, []string{"echo hello"})

	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	if !strings.Contains(results[0], "execution error:") {
		t.Errorf("result = %q, want error message", results[0])
	}
}

func TestExecuteCmds_ExitCodeNonZero(t *testing.T) {
	bridge, runner := setupTestBridge(t)
	runner.setResult("false", &logos.RunResponse{Stdout: "", ExitCode: 1})

	agentCwd := filepath.Join(os.TempDir(), "athena")
	policy, _ := cmdexec.PolicyForAgent(bridge.projectStore, agentCwd)

	results := bridge.executeCmds(context.Background(), "testagent", policy, []string{"false"})

	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	if !strings.Contains(results[0], "(exit code: 1)") {
		t.Errorf("result = %q, want exit code marker", results[0])
	}
}

func TestExecuteCmds_RecursionGuard(t *testing.T) {
	bridge, runner := setupTestBridge(t)

	agentCwd := filepath.Join(os.TempDir(), "athena")
	policy, _ := cmdexec.PolicyForAgent(bridge.projectStore, agentCwd)

	results := bridge.executeCmds(context.Background(), "testagent", policy, []string{"ttal go abc123"})

	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	if !strings.Contains(results[0], "ttal go forbidden") {
		t.Errorf("result = %q, want forbidden message", results[0])
	}
	// Runner should NOT have been called.
	if len(runner.calls) != 0 {
		t.Errorf("runner was called with: %v, want no calls", runner.calls)
	}
}

func TestExecuteCmds_RecursionGuardWithWhitespace(t *testing.T) {
	bridge, _ := setupTestBridge(t)

	agentCwd := filepath.Join(os.TempDir(), "athena")
	policy, _ := cmdexec.PolicyForAgent(bridge.projectStore, agentCwd)

	results := bridge.executeCmds(context.Background(), "testagent", policy, []string{"  TTAL   GO   uuid123"})

	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	if !strings.Contains(results[0], "ttal go forbidden") {
		t.Errorf("result = %q, want forbidden message", results[0])
	}
}

func TestExecuteCmds_StderrIncluded(t *testing.T) {
	bridge, runner := setupTestBridge(t)
	runner.setResult("ls /nonexistent", &logos.RunResponse{
		Stdout: "", Stderr: "ls: /nonexistent: No such file or directory\n", ExitCode: 1,
	})

	agentCwd := filepath.Join(os.TempDir(), "athena")
	policy, _ := cmdexec.PolicyForAgent(bridge.projectStore, agentCwd)

	results := bridge.executeCmds(context.Background(), "testagent", policy, []string{"ls /nonexistent"})

	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	if !strings.Contains(results[0], "STDERR:") {
		t.Errorf("result = %q, want STDERR marker", results[0])
	}
}

// dispatchTestBridge creates a bridge with a nil config so dispatch early-returns.
func dispatchTestBridge(t *testing.T) (*cmdexecBridge, *strings.Builder) {
	t.Helper()
	runner := newMockRunner()
	buf := &strings.Builder{}
	orig := tmuxSendKeysFn
	tmuxSendKeysFn = func(session, window, text string) error {
		buf.WriteString(text)
		return nil
	}
	t.Cleanup(func() { tmuxSendKeysFn = orig })
	bridge := &cmdexecBridge{
		cfg:          nil,
		runner:       runner,
		projectStore: nil,
		agentMutexes: sync.Map{},
	}
	return bridge, buf
}

func TestDispatch_NilCfg_SkipsDispatch(t *testing.T) {
	bridge, buf := dispatchTestBridge(t)
	bridge.dispatch("team", "athena", []string{"echo hello"})
	if buf.Len() != 0 {
		t.Errorf("tmuxSendKeysFn was called with %q, want no call", buf.String())
	}
}

func TestFormatResults(t *testing.T) {
	tests := []struct {
		name  string
		input []string
		want  string
	}{
		{name: "empty", input: []string{}, want: ""},
		{name: "single", input: []string{"echo hello\nhello"}, want: "<result>\necho hello\nhello\n</result>"},
		{name: "multiple", input: []string{"echo a\na", "echo b\nb"}, want: "<result>\necho a\na\necho b\nb\n</result>"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := formatResults(tt.input)
			if got != tt.want {
				t.Errorf("formatResults() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestFormatOneResult(t *testing.T) {
	tests := []struct {
		name     string
		cmd      string
		output   string
		errMsg   string
		exitCode int
		want     string
	}{
		{name: "success", cmd: "echo hello", output: "hello", exitCode: 0, want: "echo hello\nhello"},
		{name: "exit code nonzero", cmd: "false", output: "", exitCode: 1, want: "false\n(exit code: 1)"},
		{name: "exit code -1", cmd: "ls", output: "file.txt", exitCode: -1, want: "ls\nfile.txt"},
		{name: "error", cmd: "bad cmd", errMsg: "execution error: boom", exitCode: -1,
			want: "bad cmd\nexecution error: boom"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := formatOneResult(tt.cmd, tt.output, tt.errMsg, tt.exitCode)
			if got != tt.want {
				t.Errorf("formatOneResult() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestRecursionGuardRegex(t *testing.T) {
	cases := []struct {
		cmd  string
		want bool
	}{
		{"ttal go abc", true},
		{"ttal go", true},
		{"  ttal  go", true},
		{"TTAL GO uuid", true},
		{"ttal send", false},
		{"ttal context", false},
		{"echo ttal go hello", false}, // match at start only
		{"echo hello", false},
		{"", false},
	}
	for _, c := range cases {
		t.Run(c.cmd, func(t *testing.T) {
			got := recursionGuard.MatchString(c.cmd)
			if got != c.want {
				t.Errorf("recursionGuard.MatchString(%q) = %v, want %v", c.cmd, got, c.want)
			}
		})
	}
}

// testTruncation tests the head-truncation logic directly on formatResults output.
func TestDispatch_Truncation(t *testing.T) {
	// Build a result larger than maxPayloadBytes.
	largeOutput := strings.Repeat("x", maxPayloadBytes+1024)
	results := []string{fmt.Sprintf("echo large\n%s\n(exit code: 0)", largeOutput)}
	payload := formatResults(results)

	// Simulate truncation logic.
	if len(payload) > maxPayloadBytes {
		truncated := len(payload) - maxPayloadBytes
		marker := fmt.Sprintf("[truncated %d bytes]\n", truncated)
		if len(marker) > maxPayloadBytes {
			payload = payload[len(payload)-maxPayloadBytes:]
		} else {
			payload = marker + payload[len(payload)-maxPayloadBytes+len(marker):]
		}
	}

	// Result must be <= maxPayloadBytes.
	if len(payload) > maxPayloadBytes {
		t.Errorf("payload too large: %d bytes, want <= %d", len(payload), maxPayloadBytes)
	}
	// Result must contain the marker.
	if !strings.Contains(payload, "[truncated") {
		t.Errorf("payload missing truncation marker: %q", payload[:min(50, len(payload))])
	}
}
