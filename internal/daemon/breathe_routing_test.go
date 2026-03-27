package daemon

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/tta-lab/ttal-cli/internal/config"
	"github.com/tta-lab/ttal-cli/internal/status"
)

func TestShouldBreatheStatus(t *testing.T) {
	tests := []struct {
		name      string
		status    *status.AgentStatus
		threshold float64
		want      bool
	}{
		{"below threshold", &status.AgentStatus{ContextUsedPct: 10, UpdatedAt: time.Now()}, 40, false},
		{"at threshold", &status.AgentStatus{ContextUsedPct: 40, UpdatedAt: time.Now()}, 40, true},
		{"above threshold", &status.AgentStatus{ContextUsedPct: 75, UpdatedAt: time.Now()}, 40, true},
		{"zero ctx", &status.AgentStatus{ContextUsedPct: 0, UpdatedAt: time.Now()}, 40, false},
		{"stale status", &status.AgentStatus{ContextUsedPct: 10, UpdatedAt: time.Now().Add(-10 * time.Minute)}, 40, true},
		{"nil status", nil, 40, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := shouldBreatheStatus(tt.status, tt.threshold)
			if got != tt.want {
				t.Errorf("shouldBreatheStatus() = %v, want %v", got, tt.want)
			}
		})
	}
}

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
			resp := handleBreathe(shellCfg, tt.req)
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
		Team:    "",
		Agent:   "nonexistent-test-agent-xyz",
		Handoff: "# Handoff",
	})
	// Should fail at CWD fallback, not at team validation
	if resp.OK {
		t.Fatalf("expected OK=false (no tmux session), got OK=true")
	}
}

// TestBuildCCRestartCmd verifies that --agent flag is present and correctly interpolated.
func TestBuildCCRestartCmd(t *testing.T) {
	cmd := buildCCRestartCmd("session-abc", "sonnet", "kestrel", "")

	if !strings.Contains(cmd, "--resume session-abc") {
		t.Errorf("missing --resume flag: %q", cmd)
	}
	if !strings.Contains(cmd, "--model sonnet") {
		t.Errorf("missing --model flag: %q", cmd)
	}
	if !strings.Contains(cmd, "--agent kestrel") {
		t.Errorf("missing --agent flag: %q", cmd)
	}
	if !strings.Contains(cmd, "--dangerously-skip-permissions") {
		t.Errorf("missing --dangerously-skip-permissions flag: %q", cmd)
	}
	// Empty trigger should produce no -- separator
	if strings.Contains(cmd, "-- ") {
		t.Errorf("empty trigger should not produce -- separator: %q", cmd)
	}
}

func TestBuildCCRestartCmdWithTrigger(t *testing.T) {
	cmd := buildCCRestartCmd("session-123", "sonnet", "inke", "New task routed. Run: ttal task get abc12345")
	if !strings.Contains(cmd, "-- 'New task routed. Run: ttal task get abc12345'") {
		t.Errorf("missing trigger with -- separator: %q", cmd)
	}
}

func TestBuildCCRestartCmdEmptyTrigger(t *testing.T) {
	cmd := buildCCRestartCmd("session-123", "sonnet", "inke", "")
	if strings.Contains(cmd, "-- ") {
		t.Errorf("empty trigger should not produce -- separator: %q", cmd)
	}
}

func TestBuildCCRestartCmdApostropheEscaping(t *testing.T) {
	cmd := buildCCRestartCmd("session-abc", "sonnet", "kestrel", "it's a test")
	if !strings.Contains(cmd, "it'\\''s a test") {
		t.Errorf("apostrophe not escaped correctly: %q", cmd)
	}
}

// TestHandleSendSystemRouting verifies that From=="system" routes to handleSystemToAgent
// and not to handleAgentToAgent (which would add an [agent from:] prefix).
func TestHandleSendSystemRouting(t *testing.T) {
	// handleSend with From="system" and a known agent should return an error about
	// the agent not being found (no daemon config in test) — NOT a "send request missing"
	// error, and NOT fall through to handleAgentToAgent logic.
	//
	// We verify the routing by checking the error message: if it routes to
	// handleSystemToAgent, we get "unknown agent: <name>"; if it falls through to
	// handleAgentToAgent it would also resolve the *From* agent and return
	// "unknown agent: system" (failing on sender lookup, not recipient).
	mcfg := &config.DaemonConfig{}
	req := SendRequest{From: "system", To: "athena", Message: "run ttal skill get breathe\n\nExecute this skill now — your context window needs a refresh."} //nolint:lll
	err := handleSend(mcfg, nil, nil, nil, req)
	if err == nil {
		t.Fatal("expected error for unknown agent, got nil")
	}
	// handleSystemToAgent resolves the To agent — error must reference the recipient.
	if !strings.Contains(err.Error(), "athena") {
		t.Errorf("expected error about recipient agent 'athena', got: %v", err)
	}
	// Must NOT reference "system" as an unknown agent (which handleAgentToAgent would do
	// when it tries to resolve the From agent first).
	if strings.Contains(err.Error(), "unknown agent: system") {
		t.Errorf("routed to handleAgentToAgent instead of handleSystemToAgent: %v", err)
	}
}

// TestBuildCCRestartCmdAgentInterpolation verifies agent name is not swapped with session/model.
func TestBuildCCRestartCmdAgentInterpolation(t *testing.T) {
	cmd := buildCCRestartCmd("my-session", "opus", "athena", "")
	if !strings.Contains(cmd, "--agent athena") {
		t.Errorf("agent name not correctly interpolated, got: %q", cmd)
	}
	// Ensure model is not placed in the agent slot
	if strings.Contains(cmd, "--agent opus") {
		t.Errorf("model leaked into --agent slot: %q", cmd)
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

func TestDiaryReadToday(t *testing.T) {
	const original = "# Handoff\n\nDid some work."

	t.Run("diary not on PATH returns original handoff", func(t *testing.T) {
		t.Setenv("PATH", "/nonexistent-path-xyz")
		got := diaryReadToday("kestrel", original)
		if got != original {
			t.Errorf("expected original handoff, got %q", got)
		}
	})

	t.Run("diary read fails (non-zero exit) returns original handoff", func(t *testing.T) {
		tmp := t.TempDir()
		writeFakeDiary(t, tmp, "fail-read", "")
		t.Setenv("PATH", tmp+":"+os.Getenv("PATH"))
		got := diaryReadToday("kestrel", original)
		if got != original {
			t.Errorf("expected original handoff on read failure, got %q", got)
		}
	})

	t.Run("diary read returns empty falls back to original handoff", func(t *testing.T) {
		tmp := t.TempDir()
		writeFakeDiary(t, tmp, "empty-read", "")
		t.Setenv("PATH", tmp+":"+os.Getenv("PATH"))
		got := diaryReadToday("kestrel", original)
		if got != original {
			t.Errorf("expected original handoff on empty read, got %q", got)
		}
	})

	t.Run("diary available returns enriched handoff from read", func(t *testing.T) {
		tmp := t.TempDir()
		enriched := "# Today\nHandoff + reflection from it's a complex day"
		writeFakeDiary(t, tmp, "ok", enriched)
		t.Setenv("PATH", tmp+":"+os.Getenv("PATH"))
		got := diaryReadToday("kestrel", original)
		if got != enriched {
			t.Errorf("expected enriched handoff %q, got %q", enriched, got)
		}
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

func TestBuildBreatheEnv(t *testing.T) {
	t.Run("empty config returns identity vars only", func(t *testing.T) {
		cfg := &config.Config{}
		vars := buildBreatheEnv("kestrel", cfg)
		hasAgent := strings.Contains(strings.Join(vars, "\n"), "TTAL_AGENT_NAME=kestrel")
		if !hasAgent {
			t.Errorf("TTAL_AGENT_NAME missing from %v", vars)
		}
	})

	t.Run("includes temenos env vars", func(t *testing.T) {
		cfg := &config.Config{}
		vars := buildBreatheEnv("kestrel", cfg)
		joined := strings.Join(vars, "\n")
		if !strings.Contains(joined, "TEMENOS_WRITE=false") {
			t.Errorf("TEMENOS_WRITE=false missing from %v", vars)
		}
		if !strings.Contains(joined, "TEMENOS_PATHS=") {
			t.Errorf("TEMENOS_PATHS missing from %v", vars)
		}
		if !strings.Contains(joined, "ENABLE_TOOL_SEARCH=false") {
			t.Errorf("ENABLE_TOOL_SEARCH=false missing from %v", vars)
		}
	})

	t.Run("config with taskrc includes TASKRC var", func(t *testing.T) {
		tmp := t.TempDir()
		taskrc := filepath.Join(tmp, "taskrc")
		// Write a minimal config with both team_path and taskrc.
		cfgDir := filepath.Join(tmp, ".config", "ttal")
		if err := os.MkdirAll(cfgDir, 0o755); err != nil {
			t.Fatalf("mkdir: %v", err)
		}
		tomlContent := "[teams.default]\nteam_path = \"" + tmp + "\"\ntaskrc = \"" + taskrc + "\"\n"
		if err := os.WriteFile(filepath.Join(cfgDir, "config.toml"), []byte(tomlContent), 0o644); err != nil {
			t.Fatalf("write config: %v", err)
		}
		t.Setenv("HOME", tmp)
		cfg, err := config.Load()
		if err != nil {
			t.Fatalf("config.Load: %v", err)
		}
		vars := buildBreatheEnv("kestrel", cfg)
		joined := strings.Join(vars, "\n")
		if !strings.Contains(joined, "TASKRC="+taskrc) {
			t.Errorf("TASKRC missing or wrong in %v", vars)
		}
	})
}

// TestResolveBreatheSessions verifies session name construction and condition branching.
// Uses nonexistent session names so resolveBrCWD falls back to team path (no tmux needed).
func TestResolveBreatheSessions(t *testing.T) {
	tmp := t.TempDir()
	cfg := loadConfigWithTeamPath(t, tmp)
	const agent = "astra"
	const team = "default"

	tests := []struct {
		name           string
		req            BreatheRequest
		wantOldSession string
		wantNewSession string
	}{
		{
			name:           "self-breathe → persistent session",
			req:            BreatheRequest{Agent: agent, Team: team, SessionName: ""},
			wantOldSession: "ttal-default-" + agent,
			wantNewSession: "ttal-default-" + agent,
		},
		{
			name:           "session name override → use as old session, restart as persistent",
			req:            BreatheRequest{Agent: agent, Team: team, SessionName: "custom-session-" + agent},
			wantOldSession: "custom-session-" + agent,
			wantNewSession: "ttal-default-" + agent,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			plan, err := resolveBreatheSessions(tt.req, team, cfg)
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
