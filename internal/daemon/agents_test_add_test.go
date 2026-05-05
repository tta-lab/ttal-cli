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

func setupSpawnTest(t *testing.T, binary string) *string {
	t.Helper()
	tmp := t.TempDir()
	writeFakeBinary(t, tmp, binary)
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
	return &capturedCmd
}

func TestSpawnAgentSession_LenosVariant(t *testing.T) {
	capturedCmd := setupSpawnTest(t, "lenos")
	err := spawnAgentSession(runtime.Lenos, "sess", "test-agent", "/tmp/test", nil, "abc-123", "")
	if err != nil {
		t.Fatalf("spawnAgentSession error: %v", err)
	}
	if !strings.Contains(*capturedCmd, "lenos --agent test-agent") {
		t.Errorf("expected lenos command, got: %s", *capturedCmd)
	}
	if !strings.Contains(*capturedCmd, "--session abc-123") {
		t.Errorf("expected --session abc-123, got: %s", *capturedCmd)
	}
	if strings.Contains(*capturedCmd, "--resume") {
		t.Errorf("should not contain --resume for lenos: %s", *capturedCmd)
	}
	if strings.Contains(*capturedCmd, "claude") {
		t.Errorf("should not contain claude for lenos: %s", *capturedCmd)
	}
}

func TestSpawnAgentSession_CCVariant(t *testing.T) {
	capturedCmd := setupSpawnTest(t, "claude")
	err := spawnAgentSession(runtime.ClaudeCode, "sess", "coder", "/tmp/test", nil, "xyz-789", "")
	if err != nil {
		t.Fatalf("spawnAgentSession error: %v", err)
	}
	if !strings.Contains(*capturedCmd, "claude --dangerously-skip-permissions --agent coder") {
		t.Errorf("expected claude command, got: %s", *capturedCmd)
	}
	if !strings.Contains(*capturedCmd, "--resume xyz-789") {
		t.Errorf("expected --resume xyz-789, got: %s", *capturedCmd)
	}
	if strings.Contains(*capturedCmd, "--session") {
		t.Errorf("should not contain --session for CC: %s", *capturedCmd)
	}
	if strings.Contains(*capturedCmd, "lenos") {
		t.Errorf("should not contain lenos for CC: %s", *capturedCmd)
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
	if !strings.Contains(err.Error(), "not found in PATH") {
		t.Errorf("expected PATH error for lenos, got: %v", err)
	}
}

func TestSpawnAgentSession_TmuxError(t *testing.T) {
	setupSpawnTest(t, "claude")
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
	setupSpawnTest(t, "lenos")
	var setEnvCalls int
	tmuxSetEnvFn = func(_, _, _ string) error { setEnvCalls++; return nil }
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

func setupStatusDir(t *testing.T, tmp string) {
	t.Helper()
	sd := filepath.Join(tmp, ".ttal", "status")
	if err := os.MkdirAll(sd, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
}

func TestLastLenosSessionID_WithSessionID(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)
	setupStatusDir(t, tmp)
	if err := status.WriteAgent("default", status.AgentStatus{
		Agent: "test-agent", SessionID: "session-abc",
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
	setupStatusDir(t, tmp)
	if err := status.WriteAgent("default", status.AgentStatus{
		Agent: "test-agent", SessionID: "",
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
	setupStatusDir(t, tmp)
	if err := status.WriteAgent("default", status.AgentStatus{
		Agent: "test-agent", SessionID: "session-abc",
	}); err != nil {
		t.Fatalf("WriteAgent: %v", err)
	}
	sd := filepath.Join(tmp, ".ttal", "status")
	if err := os.Chmod(sd, 0o000); err != nil {
		t.Fatalf("chmod: %v", err)
	}
	t.Cleanup(func() {
		if err := os.Chmod(sd, 0o755); err != nil {
			t.Logf("cleanup chmod: %v", err)
		}
	})
	id := lastLenosSessionID("test-agent")
	if id != "" {
		t.Errorf("expected empty for unreadable status, got %q", id)
	}
}

func TestCollectTmuxSessions_IncludesCCAndLenos(t *testing.T) {
	tmp := t.TempDir()
	teamPath := filepath.Join(tmp, "agents")
	agentNames := []string{"cc-agent", "lenos-agent", "codex-agent"}
	for _, name := range agentNames {
		if err := os.MkdirAll(filepath.Join(teamPath, name), 0o755); err != nil {
			t.Fatalf("mkdir: %v", err)
		}
	}
	writeFile := func(p, content string) {
		if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
			t.Fatalf("write: %v", err)
		}
	}
	writeFile(filepath.Join(teamPath, "cc-agent", "AGENTS.md"), "---\n---\n# CC Agent\n")
	writeFile(filepath.Join(teamPath, "lenos-agent", "AGENTS.md"), "---\ndefault_runtime: lenos\n---\n# Lenos Agent\n")
	writeFile(filepath.Join(teamPath, "codex-agent", "AGENTS.md"), "---\ndefault_runtime: codex\n---\n# Codex Agent\n")

	cfgPath := filepath.Join(tmp, ".config", "ttal")
	if err := os.MkdirAll(cfgPath, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	writeFile(filepath.Join(cfgPath, "config.toml"), "[teams.default]\nteam_path = \""+teamPath+"\"\n")
	writeFile(filepath.Join(cfgPath, "humans.toml"),
		"[neil]\nname = \"Neil\"\ntelegram_chat_id = \"12345\"\nadmin = true\n")

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
