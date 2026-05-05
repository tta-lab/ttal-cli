package daemon

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/tta-lab/ttal-cli/internal/config"
)

const testChatID = "12345"
const testKestrelSession = "ttal-default-kestrel"

func TestHandleBreatheValidation(t *testing.T) {
	shellCfg := &config.Config{}
	tests := []struct {
		name    string
		req     BreatheRequest
		wantErr string
	}{
		{
			name:    "missing agent",
			req:     BreatheRequest{Agent: ""},
			wantErr: "missing agent name",
		},
		{
			name:    "no tmux + empty team path falls back to agent path error",
			req:     BreatheRequest{Agent: "nonexistent-test-agent-xyz"},
			wantErr: "cannot resolve agent workspace path",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resp := handleBreathe(shellCfg, tt.req, nil, nil)
			if resp.OK {
				t.Fatalf("expected OK=false, got OK=true")
			}
			if tt.wantErr != "" && !strings.Contains(resp.Error, tt.wantErr) {
				t.Errorf("expected error containing %q, got %q", tt.wantErr, resp.Error)
			}
		})
	}
}

func TestHandleBreatheTeamDefault(t *testing.T) {
	shellCfg := &config.Config{}
	resp := handleBreathe(shellCfg, BreatheRequest{
		Agent: "nonexistent-test-agent-xyz",
	}, nil, nil)
	if resp.OK {
		t.Fatalf("expected OK=false, got OK=true")
	}
}

func TestHandleSendSystemRouting(t *testing.T) {
	cfg := &config.Config{}
	req := SendRequest{From: "system", To: "athena", Message: "test"}
	err := handleSend(cfg, nil, nil, nil, req)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "athena") {
		t.Errorf("expected error about recipient agent 'athena', got: %v", err)
	}
	if strings.Contains(err.Error(), "unknown agent: system") {
		t.Errorf("routed to dispatchSend instead of dispatchSystemSend: %v", err)
	}
}

func loadConfigWithTeamPath(t *testing.T, teamPath string) *config.Config {
	t.Helper()
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)
	cfgDir := filepath.Join(tmp, ".config", "ttal")
	if err := os.MkdirAll(cfgDir, 0o755); err != nil {
		t.Fatalf("mkdir config dir: %v", err)
	}
	toml := "[teams.default]\nteam_path = \"" + teamPath + "\"\n"
	if err := os.WriteFile(filepath.Join(cfgDir, "config.toml"), []byte(toml), 0o644); err != nil {
		t.Fatalf("write config: %v", err)
	}
	humans := "[neil]\nname = \"Neil\"\ntelegram_chat_id = \"" + testChatID + "\"\nadmin = true\n"
	if err := os.WriteFile(filepath.Join(cfgDir, "humans.toml"), []byte(humans), 0o644); err != nil {
		t.Fatalf("write humans.toml: %v", err)
	}
	cfg, err := config.Load()
	if err != nil {
		t.Fatalf("config.Load: %v", err)
	}
	return cfg
}

func TestResolveBrCWD(t *testing.T) {
	t.Run("dead session + empty team path returns error", func(t *testing.T) {
		cfg := &config.Config{}
		_, _, err := resolveBrCWD("nonexistent-session-xyz", "nonexistent-agent-xyz", "nonexistent-agent-xyz", cfg)
		if err == nil {
			t.Fatal("expected error, got nil")
		}
		if !strings.Contains(err.Error(), "cannot resolve agent workspace path") {
			t.Errorf("unexpected error: %v", err)
		}
	})
	t.Run("dead session + configured team path returns agent path", func(t *testing.T) {
		tmp := t.TempDir()
		cfg := loadConfigWithTeamPath(t, tmp)
		agent := "testagent"
		cwd, sessionAlive, err := resolveBrCWD("nonexistent-session-xyz", agent, agent, cfg)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if sessionAlive {
			t.Errorf("expected sessionAlive=false for nonexistent session")
		}
		want := filepath.Join(tmp, agent)
		if cwd != want {
			t.Errorf("cwd = %q, want %q", cwd, want)
		}
	})
}

func TestResolveBreatheSessions(t *testing.T) {
	tmp := t.TempDir()
	cfg := loadConfigWithTeamPath(t, tmp)
	const agent = "astra"
	tests := []struct {
		name           string
		req            BreatheRequest
		wantOldSession string
		wantNewSession string
	}{
		{
			name:           "self-breathe uses persistent session",
			req:            BreatheRequest{Agent: agent, SessionName: ""},
			wantOldSession: "ttal-default-" + agent,
			wantNewSession: "ttal-default-" + agent,
		},
		{
			name:           "session name override uses custom old, persistent new",
			req:            BreatheRequest{Agent: agent, SessionName: "custom-" + agent},
			wantOldSession: "custom-" + agent,
			wantNewSession: "ttal-default-" + agent,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			plan, err := resolveBreatheSessions(tt.req, cfg)
			if err != nil {
				t.Fatalf("resolveBreatheSessions error: %v", err)
			}
			if plan.oldSessionName != tt.wantOldSession {
				t.Errorf("oldSessionName = %q, want %q", plan.oldSessionName, tt.wantOldSession)
			}
			if plan.newSessionName != tt.wantNewSession {
				t.Errorf("newSessionName = %q, want %q", plan.newSessionName, tt.wantNewSession)
			}
		})
	}
}

func writeFakeBinary(t *testing.T, tmpDir, name string) {
	t.Helper()
	binDir := filepath.Join(tmpDir, "bin")
	if err := os.MkdirAll(binDir, 0o755); err != nil {
		t.Fatalf("mkdir bin: %v", err)
	}
	path := filepath.Join(binDir, name)
	if err := os.WriteFile(path, []byte("#!/bin/sh\nexit 0\n"), 0o755); err != nil {
		t.Fatalf("write fake %s: %v", name, err)
	}
}

func TestHandleBreathe_RespawnsCleanly(t *testing.T) {
	tmp := t.TempDir()
	writeFakeBinary(t, tmp, "claude")
	t.Setenv("PATH", filepath.Join(tmp, "bin"))

	origKill := tmuxKillSessionFn
	origExists := tmuxSessionExistsFn
	origNew := tmuxNewSessionFn
	origSet := tmuxSetEnvFn
	t.Cleanup(func() {
		tmuxKillSessionFn = origKill
		tmuxSessionExistsFn = origExists
		tmuxNewSessionFn = origNew
		tmuxSetEnvFn = origSet
	})

	var killCalled bool
	var killedSession string
	tmuxKillSessionFn = func(session string) error {
		killCalled = true
		killedSession = session
		return nil
	}
	tmuxSessionExistsFn = func(name string) bool {
		return name == testKestrelSession
	}
	var capturedCmd string
	tmuxNewSessionFn = func(_, _, _, cmd string) error {
		capturedCmd = cmd
		return nil
	}
	tmuxSetEnvFn = func(_, _, _ string) error { return nil }

	cfg := loadConfigWithTeamPath(t, t.TempDir())

	resp := handleBreathe(cfg, BreatheRequest{
		Agent:       "kestrel",
		SessionName: testKestrelSession,
	}, nil, nil)

	if !resp.OK {
		t.Fatalf("handleBreathe returned OK=false: %s", resp.Error)
	}
	if !killCalled {
		t.Error("expected KillSession called (session alive)")
	}
	if killedSession != testKestrelSession {
		t.Errorf("killed session = %q, want %q", killedSession, testKestrelSession)
	}
	if !strings.Contains(capturedCmd, "claude --dangerously-skip-permissions --agent kestrel") {
		t.Errorf("expected claude command, got: %s", capturedCmd)
	}
	if strings.Contains(capturedCmd, "lenos") {
		t.Errorf("should not contain lenos for CC: %s", capturedCmd)
	}
}

func TestHandleBreathe_DeadSessionPath(t *testing.T) {
	tmp := t.TempDir()
	writeFakeBinary(t, tmp, "claude")
	t.Setenv("PATH", filepath.Join(tmp, "bin"))

	origKill := tmuxKillSessionFn
	origExists := tmuxSessionExistsFn
	origNew := tmuxNewSessionFn
	origSet := tmuxSetEnvFn
	t.Cleanup(func() {
		tmuxKillSessionFn = origKill
		tmuxSessionExistsFn = origExists
		tmuxNewSessionFn = origNew
		tmuxSetEnvFn = origSet
	})

	var killCalled bool
	tmuxKillSessionFn = func(_ string) error {
		killCalled = true
		return nil
	}
	tmuxSessionExistsFn = func(_ string) bool { return false }
	var capturedCmd string
	tmuxNewSessionFn = func(_, _, _, cmd string) error {
		capturedCmd = cmd
		return nil
	}
	tmuxSetEnvFn = func(_, _, _ string) error { return nil }

	cfg := loadConfigWithTeamPath(t, t.TempDir())

	resp := handleBreathe(cfg, BreatheRequest{
		Agent:       "kestrel",
		SessionName: testKestrelSession,
	}, nil, nil)

	if !resp.OK {
		t.Fatalf("handleBreathe returned OK=false: %s", resp.Error)
	}
	if killCalled {
		t.Error("expected KillSession NOT called (dead session)")
	}
	if !strings.Contains(capturedCmd, "claude --dangerously-skip-permissions --agent kestrel") {
		t.Errorf("expected claude command, got: %s", capturedCmd)
	}
}

func TestHandleBreathe_LenosRespawn(t *testing.T) {
	tmp := t.TempDir()
	writeFakeBinary(t, tmp, "lenos")
	writeFakeBinary(t, tmp, "claude")
	t.Setenv("PATH", filepath.Join(tmp, "bin"))

	origKill := tmuxKillSessionFn
	origExists := tmuxSessionExistsFn
	origNew := tmuxNewSessionFn
	origSet := tmuxSetEnvFn
	t.Cleanup(func() {
		tmuxKillSessionFn = origKill
		tmuxSessionExistsFn = origExists
		tmuxNewSessionFn = origNew
		tmuxSetEnvFn = origSet
	})

	tmuxKillSessionFn = func(_ string) error { return nil }
	tmuxSessionExistsFn = func(_ string) bool { return true }
	var capturedCmd string
	tmuxNewSessionFn = func(_, _, _, cmd string) error {
		capturedCmd = cmd
		return nil
	}
	tmuxSetEnvFn = func(_, _, _ string) error { return nil }

	// Create agent dir BEFORE loading config so agentfs discovers it.
	tmpHome := t.TempDir()
	t.Setenv("HOME", tmpHome)
	cfgDir := filepath.Join(tmpHome, ".config", "ttal")
	if err := os.MkdirAll(cfgDir, 0o755); err != nil {
		t.Fatalf("mkdir config dir: %v", err)
	}
	teamPath := t.TempDir()
	agentPath := filepath.Join(teamPath, "lenos-agent")
	if err := os.MkdirAll(agentPath, 0o755); err != nil {
		t.Fatalf("mkdir agent: %v", err)
	}
	agentsMD := "---\ndefault_runtime: lenos\n---\n# Lenos Agent\n"
	agentsFile := filepath.Join(agentPath, "AGENTS.md")
	if err := os.WriteFile(agentsFile, []byte(agentsMD), 0o644); err != nil {
		t.Fatalf("write AGENTS.md: %v", err)
	}

	toml := "[teams.default]\nteam_path = \"" + teamPath + "\"\n"
	if err := os.WriteFile(filepath.Join(cfgDir, "config.toml"), []byte(toml), 0o644); err != nil {
		t.Fatalf("write config: %v", err)
	}
	humans := "[neil]\nname = \"Neil\"\ntelegram_chat_id = \"" + testChatID + "\"\nadmin = true\n"
	if err := os.WriteFile(filepath.Join(cfgDir, "humans.toml"), []byte(humans), 0o644); err != nil {
		t.Fatalf("write humans.toml: %v", err)
	}
	cfg, err := config.Load()
	if err != nil {
		t.Fatalf("config.Load: %v", err)
	}

	resp := handleBreathe(cfg, BreatheRequest{
		Agent:       "lenos-agent",
		SessionName: "ttal-default-lenos-agent",
	}, cfg, nil)

	if !resp.OK {
		t.Fatalf("handleBreathe returned OK=false: %s", resp.Error)
	}
	if !strings.Contains(capturedCmd, "lenos --agent lenos-agent") {
		t.Errorf("expected lenos command, got: %s", capturedCmd)
	}
	if strings.Contains(capturedCmd, "claude --dangerously-skip-permissions") {
		t.Errorf("should not contain claude for lenos: %s", capturedCmd)
	}
}

func TestResolveBrCWD_LoadAllPath(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)
	cfgDir := filepath.Join(tmp, ".config", "ttal")
	if err := os.MkdirAll(cfgDir, 0o755); err != nil {
		t.Fatalf("mkdir config dir: %v", err)
	}
	toml := "[teams.default]\nteam_path = \"" + tmp + "\"\n"
	if err := os.WriteFile(filepath.Join(cfgDir, "config.toml"), []byte(toml), 0o644); err != nil {
		t.Fatalf("write config: %v", err)
	}
	humans := "[neil]\nname = \"Neil\"\ntelegram_chat_id = \"" + testChatID + "\"\nadmin = true\n"
	if err := os.WriteFile(filepath.Join(cfgDir, "humans.toml"), []byte(humans), 0o644); err != nil {
		t.Fatalf("write humans.toml: %v", err)
	}
	cfg, err := config.Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	agent := "athena"
	cwd, sessionAlive, err := resolveBrCWD("nonexistent-session-xyz", agent, agent, cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if sessionAlive {
		t.Error("expected sessionAlive=false for nonexistent session")
	}
	want := filepath.Join(tmp, agent)
	if cwd != want {
		t.Errorf("cwd = %q, want %q", cwd, want)
	}
}
