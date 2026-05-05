package daemon

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/tta-lab/ttal-cli/internal/config"
	"github.com/tta-lab/ttal-cli/internal/runtime"
	"github.com/tta-lab/ttal-cli/internal/status"
)

func TestSpawnAgentSession_LenosVariant(t *testing.T) {
	tmp := t.TempDir()
	writeFakeBinary(t, tmp, "lenos")
	t.Setenv("PATH", filepath.Join(tmp, "bin"))
	origNew := tmuxNewSessionFn
	origSet := tmuxSetEnvFn
	t.Cleanup(func() {
		tmuxNewSessionFn = origNew
		tmuxSetEnvFn = origSet
	})
	var capturedCmd string
	tmuxNewSessionFn = func(_, _, _, cmd string) error {
		capturedCmd = cmd
		return nil
	}
	tmuxSetEnvFn = func(_, _, _ string) error { return nil }
	err := spawnAgentSession(runtime.Lenos, "sess", "test-agent", "/tmp/test", nil, "abc-123", "")
	if err != nil {
		t.Fatalf("spawnAgentSession error: %v", err)
	}
	if !strings.Contains(capturedCmd, "lenos --agent test-agent") {
		t.Errorf("expected lenos command, got: %s", capturedCmd)
	}
	if !strings.Contains(capturedCmd, "--session abc-123") {
		t.Errorf("expected --session abc-123, got: %s", capturedCmd)
	}
	if strings.Contains(capturedCmd, "--resume") {
		t.Errorf("should not contain --resume for lenos: %s", capturedCmd)
	}
	if strings.Contains(capturedCmd, "claude") {
		t.Errorf("should not contain claude for lenos: %s", capturedCmd)
	}
}

func TestSpawnAgentSession_CCVariant(t *testing.T) {
	tmp := t.TempDir()
	writeFakeBinary(t, tmp, "claude")
	t.Setenv("PATH", filepath.Join(tmp, "bin"))
	origNew := tmuxNewSessionFn
	origSet := tmuxSetEnvFn
	t.Cleanup(func() {
		tmuxNewSessionFn = origNew
		tmuxSetEnvFn = origSet
	})
	var capturedCmd string
	tmuxNewSessionFn = func(_, _, _, cmd string) error {
		capturedCmd = cmd
		return nil
	}
	tmuxSetEnvFn = func(_, _, _ string) error { return nil }
	err := spawnAgentSession(runtime.ClaudeCode, "sess", "coder", "/tmp/test", nil, "xyz-789", "")
	if err != nil {
		t.Fatalf("spawnAgentSession error: %v", err)
	}
	if !strings.Contains(capturedCmd, "claude --dangerously-skip-permissions --agent coder") {
		t.Errorf("expected claude command, got: %s", capturedCmd)
	}
	if !strings.Contains(capturedCmd, "--resume xyz-789") {
		t.Errorf("expected --resume xyz-789, got: %s", capturedCmd)
	}
	if strings.Contains(capturedCmd, "--session") {
		t.Errorf("should not contain --session for CC: %s", capturedCmd)
	}
	if strings.Contains(capturedCmd, "lenos") {
		t.Errorf("should not contain lenos for CC: %s", capturedCmd)
	}
}

func TestSpawnAgentSession_LenosNotInPath(t *testing.T) {
	t.Setenv("PATH", "/nonexistent")
	origNew := tmuxNewSessionFn
	t.Cleanup(func() { tmuxNewSessionFn = origNew })
	tmuxNewSessionFn = func(_, _, _, _ string) error { return nil }
	err := spawnAgentSession(runtime.Lenos, "sess", "test-agent", "/tmp", nil, "", "")
	if err == nil {
		t.Fatal("expected error for missing lenos binary, got nil")
	}
	if !strings.Contains(err.Error(), "lenos") || !strings.Contains(err.Error(), "not found in PATH") {
		t.Errorf("expected PATH error for lenos, got: %v", err)
	}
}

func TestSpawnAgentSession_TmuxError(t *testing.T) {
	tmp := t.TempDir()
	writeFakeBinary(t, tmp, "claude")
	t.Setenv("PATH", filepath.Join(tmp, "bin"))
	origNew := tmuxNewSessionFn
	t.Cleanup(func() { tmuxNewSessionFn = origNew })
	tmuxNewSessionFn = func(_, _, _, _ string) error {
		return os.ErrNotExist
	}
	err := spawnAgentSession(runtime.ClaudeCode, "sess", "coder", "/tmp", nil, "", "")
	if err == nil {
		t.Fatal("expected error from tmuxNewSessionFn, got nil")
	}
	if !strings.Contains(err.Error(), "failed to create session") {
		t.Errorf("expected 'failed to create session' wrapping, got: %v", err)
	}
}

func TestSpawnAgentSession_EmptyEnv(t *testing.T) {
	tmp := t.TempDir()
	writeFakeBinary(t, tmp, "lenos")
	t.Setenv("PATH", filepath.Join(tmp, "bin"))
	origNew := tmuxNewSessionFn
	origSet := tmuxSetEnvFn
	t.Cleanup(func() {
		tmuxNewSessionFn = origNew
		tmuxSetEnvFn = origSet
	})
	var setEnvCalls int
	tmuxNewSessionFn = func(_, _, _, _ string) error { return nil }
	tmuxSetEnvFn = func(_, _, _ string) error {
		setEnvCalls++
		return nil
	}
	if err := spawnAgentSession(runtime.Lenos, "sess", "test-agent", "/tmp", nil, "", ""); err != nil {
		t.Fatalf("spawnAgentSession error: %v", err)
	}
	if setEnvCalls != 0 {
		t.Errorf("expected 0 tmuxSetEnvFn calls with nil env, got %d", setEnvCalls)
	}
}

func TestLastLenosSessionID_ColdStart(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)
	id := lastLenosSessionID("test-agent")
	if id != "" {
		t.Errorf("expected empty for cold start, got %q", id)
	}
}

func TestLastLenosSessionID_WithSessionID(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)
	statusDir := filepath.Join(tmp, ".ttal", "status")
	if err := os.MkdirAll(statusDir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := status.WriteAgent("default", status.AgentStatus{
		Agent:     "test-agent",
		SessionID: "session-abc",
	}); err != nil {
		t.Fatalf("WriteAgent: %v", err)
	}
	id := lastLenosSessionID("test-agent")
	if id != "session-abc" {
		t.Errorf("expected 'session-abc', got %q", id)
	}
}

func TestLastLenosSessionID_EmptySessionID(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)
	statusDir := filepath.Join(tmp, ".ttal", "status")
	if err := os.MkdirAll(statusDir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := status.WriteAgent("default", status.AgentStatus{
		Agent:     "test-agent",
		SessionID: "",
	}); err != nil {
		t.Fatalf("WriteAgent: %v", err)
	}
	id := lastLenosSessionID("test-agent")
	if id != "" {
		t.Errorf("expected empty for empty SessionID, got %q", id)
	}
}

func TestLastLenosSessionID_UnreadableFile(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)
	statusDir := filepath.Join(tmp, ".ttal", "status")
	if err := os.MkdirAll(statusDir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := status.WriteAgent("default", status.AgentStatus{
		Agent:     "test-agent",
		SessionID: "session-abc",
	}); err != nil {
		t.Fatalf("WriteAgent: %v", err)
	}
	os.Chmod(statusDir, 0o000)
	t.Cleanup(func() { os.Chmod(statusDir, 0o755) })
	id := lastLenosSessionID("test-agent")
	if id != "" {
		t.Errorf("expected empty for unreadable status, got %q", id)
	}
}

func TestCollectTmuxSessions_IncludesCCAndLenos(t *testing.T) {
	tmp := t.TempDir()
	teamPath := filepath.Join(tmp, "agents")
	for _, name := range []string{"cc-agent", "lenos-agent", "codex-agent"} {
		os.MkdirAll(filepath.Join(teamPath, name), 0o755)
	}
	os.WriteFile(filepath.Join(teamPath, "cc-agent", "AGENTS.md"), []byte("---\n---\n# CC Agent\n"), 0o644)
	os.WriteFile(filepath.Join(teamPath, "lenos-agent", "AGENTS.md"), []byte("---\ndefault_runtime: lenos\n---\n# Lenos Agent\n"), 0o644)
	os.WriteFile(filepath.Join(teamPath, "codex-agent", "AGENTS.md"), []byte("---\ndefault_runtime: codex\n---\n# Codex Agent\n"), 0o644)
	cfgPath := filepath.Join(tmp, ".config", "ttal")
	os.MkdirAll(cfgPath, 0o755)
	os.WriteFile(filepath.Join(cfgPath, "config.toml"), []byte("[teams.default]\nteam_path = \""+teamPath+"\"\n"), 0o644)
	os.WriteFile(filepath.Join(cfgPath, "humans.toml"), []byte("[neil]\nname = \"Neil\"\ntelegram_chat_id = \"12345\"\nadmin = true\n"), 0o644)
	t.Setenv("HOME", tmp)
	cfg, err := config.Load()
	if err != nil {
		t.Fatalf("config.Load: %v", err)
	}
	origExists := tmuxSessionExistsFn
	t.Cleanup(func() { tmuxSessionExistsFn = origExists })
	tmuxSessionExistsFn = func(name string) bool {
		return name == "ttal-default-cc-agent" || name == "ttal-default-lenos-agent"
	}
	sessions := collectTmuxSessions(cfg)
	if len(sessions) != 2 {
		t.Fatalf("expected 2 sessions (CC + Lenos), got %d: %v", len(sessions), sessions)
	}
	hasCC := false
	hasLenos := false
	for _, s := range sessions {
		if s == "ttal-default-cc-agent" {
			hasCC = true
		}
		if s == "ttal-default-lenos-agent" {
			hasLenos = true
		}
	}
	if !hasCC {
		t.Errorf("expected CC session in results: %v", sessions)
	}
	if !hasLenos {
		t.Errorf("expected Lenos session in results: %v", sessions)
	}
}
