package daemon

import (
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/tta-lab/ttal-cli/internal/config"
	"github.com/tta-lab/ttal-cli/internal/launchcmd"
)

// testChatID is a placeholder chat ID used across fixture humans.toml entries.
// goconst linter requires this to be a constant rather than repeated string literals.
const testChatID = "12345"

func TestHandleBreatheValidation(t *testing.T) {
	shellCfg := &config.Config{}

	tests := []struct {
		name    string
		req     BreatheRequest
		wantErr string
	}{
		{
			name:    "missing agent",
			req:     BreatheRequest{Agent: "", Handoff: "# Handoff"},
			wantErr: "missing agent name",
		},
		{
			name:    "empty handoff",
			req:     BreatheRequest{Agent: "kestrel", Handoff: ""},
			wantErr: "empty handoff prompt",
		},
		{
			name:    "no tmux + empty team path falls back to agent path error",
			req:     BreatheRequest{Agent: "nonexistent-test-agent-xyz", Handoff: "# Handoff\n\nNext steps: continue"},
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

	// team="" should default without panicking — it will fail at CWD fallback
	// (shellCfg has no resolved team path, so AgentPath returns "").
	resp := handleBreathe(shellCfg, BreatheRequest{
		Agent:   "nonexistent-test-agent-xyz",
		Handoff: "# Handoff",
	}, nil, nil)
	// Should fail at CWD fallback, not at team validation
	if resp.OK {
		t.Fatalf("expected OK=false (no tmux session), got OK=true")
	}
}

// TestHandleSendSystemRouting verifies that From=="system" routes to dispatchSystemSend
// and not to dispatchSend (which would add an [agent from:] prefix).
func TestHandleSendSystemRouting(t *testing.T) {
	// handleSend with From="system" and a known agent should return an error about
	// the agent not being found (no daemon config in test) — NOT a "send request missing"
	// error, and NOT fall through to dispatchSend logic.
	//
	// We verify the routing by checking the error message: if it routes to
	// dispatchSystemSend, we get "unknown agent: <name>"; if it falls through to
	// dispatchSend it would also resolve the *From* agent and return
	// "unknown agent: system" (failing on sender lookup, not recipient).
	cfg := &config.Config{}
	req := SendRequest{From: "system", To: "athena", Message: "run skill get breathe\n\nExecute this skill now — your context window needs a refresh."} //nolint:lll
	err := handleSend(cfg, nil, nil, nil, req)
	if err == nil {
		t.Fatal("expected error for unknown agent, got nil")
	}
	// dispatchSystemSend resolves the To agent — error must reference the recipient.
	if !strings.Contains(err.Error(), "athena") {
		t.Errorf("expected error about recipient agent 'athena', got: %v", err)
	}
	// Must NOT reference "system" as an unknown agent (which dispatchSend would do
	// when it tries to resolve the From agent first).
	if strings.Contains(err.Error(), "unknown agent: system") {
		t.Errorf("routed to dispatchSend instead of dispatchSystemSend: %v", err)
	}
}

// writeFakeDiary writes a fake diary shell script to tmpDir.
// readOutput is written to a side file so the script can cat it safely —
// avoids shell quoting issues with arbitrary content.
//
// Modes:
//   - "ok"          — append succeeds, read returns readOutput
//   - "fail-append" — append exits non-zero with error on stderr
//   - "empty-read"  — append succeeds, read exits 0 with no output
//   - "fail-read"   — append succeeds, read exits non-zero with error on stderr
func writeFakeDiary(t *testing.T, tmpDir, mode, readOutput string) {
	t.Helper()
	outputFile := filepath.Join(tmpDir, "diary-read-output")
	if err := os.WriteFile(outputFile, []byte(readOutput), 0o644); err != nil {
		t.Fatalf("write diary read output file: %v", err)
	}

	var script string
	switch mode {
	case "ok":
		script = "#!/bin/sh\n" +
			"if [ \"$2\" = \"append\" ]; then cat > /dev/null; exit 0; fi\n" +
			"if [ \"$2\" = \"read\" ]; then cat '" + outputFile + "'; exit 0; fi\n" +
			"exit 1\n"
	case "fail-append":
		script = "#!/bin/sh\necho 'text required for append command' >&2; exit 1\n"
	case "empty-read":
		script = "#!/bin/sh\n" +
			"if [ \"$2\" = \"append\" ]; then cat > /dev/null; exit 0; fi\n" +
			"if [ \"$2\" = \"read\" ]; then exit 0; fi\n" +
			"exit 1\n"
	case "fail-read":
		script = "#!/bin/sh\n" +
			"if [ \"$2\" = \"append\" ]; then cat > /dev/null; exit 0; fi\n" +
			"if [ \"$2\" = \"read\" ]; then echo 'diary read error' >&2; exit 1; fi\n" +
			"exit 1\n"
	}
	diaryPath := filepath.Join(tmpDir, "diary")
	if err := os.WriteFile(diaryPath, []byte(script), 0o755); err != nil {
		t.Fatalf("write fake diary: %v", err)
	}
}

func TestDiaryAppendHandoff(t *testing.T) {
	const handoff = "# Handoff\n\nDid some work."

	t.Run("diary not on PATH is a no-op", func(t *testing.T) {
		t.Setenv("PATH", "/nonexistent-path-xyz")
		// Must not panic — no return value to assert.
		diaryAppendHandoff("kestrel", handoff)
	})

	t.Run("diary append failure does not panic", func(t *testing.T) {
		tmp := t.TempDir()
		writeFakeDiary(t, tmp, "fail-append", "")
		t.Setenv("PATH", tmp+":"+os.Getenv("PATH"))
		diaryAppendHandoff("kestrel", handoff)
	})
}

// loadConfigWithTeamPath writes a minimal config.toml to a temp HOME and loads it.
// Used to test code paths that require a resolved team path.
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

// TestResolveBreatheSessions verifies session name construction and condition branching.
// Uses nonexistent session names so resolveBrCWD falls back to team path (no tmux needed).
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
			name:           "self-breathe → persistent session",
			req:            BreatheRequest{Agent: agent, SessionName: ""},
			wantOldSession: "ttal-default-" + agent,
			wantNewSession: "ttal-default-" + agent,
		},
		{
			name:           "session name override → use as old session, restart as persistent",
			req:            BreatheRequest{Agent: agent, SessionName: "custom-session-" + agent},
			wantOldSession: "custom-session-" + agent,
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

// TestHandleBreathe_SendsContinueWithTask verifies that after sending /clear, the daemon
// sends the configured start trigger. This ensures agents get fresh context after a
// breathe cycle.
func TestHandleBreathe_SendsContinueWithTask(t *testing.T) {
	// Use a real temp directory as the team path so cfg.AgentPath() resolves
	// and resolveAgentModel can find the agent status file.
	tmpTeamDir := t.TempDir()

	// Construct a minimal config with resolvedTeamPath set.
	// resolvedTeamPath is a private field — we can't set it directly on &config.Config{}.
	// Use config.LoadAll() which properly populates all resolved fields.
	// config.LoadAll() reads XDG_CONFIG_HOME, so we write the config there.
	home, err := os.UserHomeDir()
	if err != nil {
		t.Fatalf("UserHomeDir: %v", err)
	}
	configDir := filepath.Join(home, ".config", "ttal")
	if err := os.MkdirAll(configDir, 0o755); err != nil {
		t.Fatalf("mkdir config dir: %v", err)
	}

	// Back up existing config so we can restore it.
	configPath := filepath.Join(configDir, "config.toml")
	var backup []byte
	if oldCfg, err := os.ReadFile(configPath); err == nil {
		backup = oldCfg
	}
	t.Cleanup(func() {
		if backup != nil {
			os.WriteFile(configPath, backup, 0o644)
		} else {
			os.Remove(configPath)
		}
	})

	configYAML := "default_team = \"\"\n[teams.default]\nteam_path = \"" + tmpTeamDir + "\"\nchat_id = \"12345\"\n"
	if err := os.WriteFile(configPath, []byte(configYAML), 0o644); err != nil {
		t.Fatalf("write config.toml: %v", err)
	}

	// humans.toml is now required by config.Load() — write a minimal fixture.
	humansPath := filepath.Join(configDir, "humans.toml")
	var humansBackup []byte
	if oldHumans, err := os.ReadFile(humansPath); err == nil {
		humansBackup = oldHumans
	}
	t.Cleanup(func() {
		if humansBackup != nil {
			os.WriteFile(humansPath, humansBackup, 0o644)
		} else {
			os.Remove(humansPath)
		}
	})
	humansContent := "[neil]\nname = \"Neil\"\ntelegram_chat_id = \"" + testChatID + "\"\nadmin = true\n"
	if err := os.WriteFile(humansPath, []byte(humansContent), 0o644); err != nil {
		t.Fatalf("write humans.toml: %v", err)
	}

	cfg, err := config.Load()
	if err != nil {
		t.Fatalf("config.Load: %v", err)
	}

	origSendKeys := tmuxSendKeysFn
	origSessionExists := tmuxSessionExistsFn
	t.Cleanup(func() {
		tmuxSendKeysFn = origSendKeys
		tmuxSessionExistsFn = origSessionExists
	})

	var mu sync.Mutex
	var sendCalls []struct {
		session string
		window  string
		text    string
	}

	tmuxSendKeysFn = func(session, window, text string) error {
		mu.Lock()
		defer mu.Unlock()
		sendCalls = append(sendCalls, struct {
			session string
			window  string
			text    string
		}{session, window, text})
		return nil
	}
	// Session exists → /clear path taken
	tmuxSessionExistsFn = func(name string) bool {
		return name == "ttal-default-kestrel"
	}

	// Use SessionName to bypass GetPaneCwd (no real tmux available in test).
	resp := handleBreathe(cfg, BreatheRequest{
		Agent:       "kestrel",
		Handoff:     "# Handoff\n\nNext steps: continue",
		SessionName: "ttal-default-kestrel",
	}, nil, nil)

	// handleBreathe returns immediately (trigger is sent async).
	if !resp.OK {
		t.Fatalf("handleBreathe returned OK=false: %s", resp.Error)
	}

	// Wait for the async trigger to fire (clearSettleDelay = 500ms).
	time.Sleep(600 * time.Millisecond)

	mu.Lock()
	defer mu.Unlock()

	// Should have exactly two SendKeys calls: /clear then the configured start trigger.
	if len(sendCalls) < 2 {
		t.Fatalf("expected at least 2 SendKeys calls, got %d: %v", len(sendCalls), sendCalls)
	}

	if sendCalls[0].text != "/clear" {
		t.Errorf("first SendKeys call text = %q, want %q", sendCalls[0].text, "/clear")
	}

	// All breathe paths now return the single ttal context trigger.
	if sendCalls[1].text != breatheStartTriggerFallback {
		t.Errorf("second SendKeys call text = %q, want %q", sendCalls[1].text, breatheStartTriggerFallback)
	}
}

// TestBuildBreatheStartTrigger tests the buildBreatheStartTrigger function.
func TestBuildBreatheStartTrigger(t *testing.T) {
	// Guard: the fallback must equal launchcmd.ContextTrigger.
	if breatheStartTriggerFallback != launchcmd.ContextTrigger {
		t.Errorf("fallback not ttal context trigger: got %q", breatheStartTriggerFallback)
	}

	// Test: empty agent name returns fallback
	result := buildBreatheStartTrigger("")
	if result != breatheStartTriggerFallback {
		t.Errorf("empty agent name: got %q, want fallback %q", result, breatheStartTriggerFallback)
	}

	// Test: nonexistent agent returns fallback (no valid config)
	result = buildBreatheStartTrigger("nonexistent-agent")
	if result != breatheStartTriggerFallback {
		t.Errorf("nonexistent agent: got %q, want fallback %q", result, breatheStartTriggerFallback)
	}
}

// TestResolveBrCWD_LoadAllPath verifies that resolveBrCWD resolves agent paths
// correctly when passed the Global config from a DaemonConfig produced by
// LoadAll(). This is the path the daemon actually uses (not config.Load()).
// Regression test for the resolvedTeamPath mirror bug.
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
