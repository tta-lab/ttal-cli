package cmd

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/spf13/cobra"
)

// captureContextOutput runs runContext and captures stdout.
// stdinJSON is optional hook input JSON to provide via stdin.
func captureContextOutput(t *testing.T, stdinJSON ...string) string {
	t.Helper()

	// Inject stdin if provided.
	if len(stdinJSON) > 0 && stdinJSON[0] != "" {
		oldStdin := os.Stdin
		pr, pw, err := os.Pipe()
		if err != nil {
			t.Fatalf("stdin pipe: %v", err)
		}
		if _, err := pw.WriteString(stdinJSON[0]); err != nil {
			t.Fatalf("write stdin: %v", err)
		}
		pw.Close()
		os.Stdin = pr
		defer func() { os.Stdin = oldStdin }()
	}

	// Redirect stdout by capturing via cobra output
	buf := &bytes.Buffer{}
	contextCmd.SetOut(buf)
	defer contextCmd.SetOut(nil)

	// Use the existing runContext but redirect stdout temporarily
	old := os.Stdout
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("pipe: %v", err)
	}
	os.Stdout = w

	_ = runContext(&cobra.Command{}, []string{})

	w.Close()
	os.Stdout = old

	var out bytes.Buffer
	if _, err := out.ReadFrom(r); err != nil {
		t.Fatalf("read pipe: %v", err)
	}
	return out.String()
}

// TestRunContext_NonAgentSession verifies non-agent sessions output {}.
func TestRunContext_NonAgentSession(t *testing.T) {
	// No agent_type in hook input — should be a no-op.
	output := captureContextOutput(t, `{"cwd":"/some/random/dir"}`)
	output = trimNewlines(output)
	if output != "{}" {
		t.Errorf("expected {} for non-agent session, got %q", output)
	}
}

// TestRunContext_EmptyStdin verifies empty stdin (no hook input) produces {}.
func TestRunContext_EmptyStdin(t *testing.T) {
	// Wire up a closed pipe so readHookInput sees EOF with zero bytes.
	pr, pw, err := os.Pipe()
	if err != nil {
		t.Fatalf("pipe: %v", err)
	}
	pw.Close()
	oldStdin := os.Stdin
	os.Stdin = pr
	defer func() { os.Stdin = oldStdin }()

	output := captureContextOutput(t)
	output = trimNewlines(output)
	if output != "{}" {
		t.Errorf("expected {} for empty stdin, got %q", output)
	}
}

// TestRunContext_MalformedStdin verifies malformed JSON stdin produces {}.
func TestRunContext_MalformedStdin(t *testing.T) {
	output := captureContextOutput(t, `{"bad json:}`)
	output = trimNewlines(output)
	if output != "{}" {
		t.Errorf("expected {} for malformed stdin, got %q", output)
	}
}

// TestRunContext_AgentWithContextTemplate verifies agent session with context template
// outputs valid JSON with additionalContext.
func TestRunContext_AgentWithContextTemplate(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)

	cfgDir := filepath.Join(tmp, ".config", "ttal")
	if err := os.MkdirAll(cfgDir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	// Write config with a context template containing a $ cmd line.
	promptsToml := "context = \"$ echo context-from-hook\"\n"
	if err := os.WriteFile(filepath.Join(cfgDir, "prompts.toml"), []byte(promptsToml), 0o644); err != nil {
		t.Fatalf("write prompts.toml: %v", err)
	}
	configToml := "[teams.default]\nteam_path = \"" + tmp + "\"\n"
	if err := os.WriteFile(filepath.Join(cfgDir, "config.toml"), []byte(configToml), 0o644); err != nil {
		t.Fatalf("write config.toml: %v", err)
	}

	hookInput := testHookInputKestrel
	output := captureContextOutput(t, hookInput)
	output = trimNewlines(output)

	// Output must always be valid JSON.
	var resp ccHookResponse
	if err := json.Unmarshal([]byte(output), &resp); err != nil {
		t.Fatalf("output is not valid JSON: %v\noutput: %q", err, output)
	}
	if resp.HookSpecificOutput == nil {
		t.Fatalf("expected hookSpecificOutput in output, got: %q", output)
	}
	if resp.HookSpecificOutput.HookEventName != "SessionStart" {
		t.Errorf("expected hookEventName=SessionStart, got: %q", resp.HookSpecificOutput.HookEventName)
	}
	if !strings.Contains(resp.HookSpecificOutput.AdditionalContext, "context-from-hook") {
		t.Errorf("expected 'context-from-hook' in additionalContext, got: %q",
			resp.HookSpecificOutput.AdditionalContext)
	}
}

// TestRunContext_MissingConfig verifies missing config.toml produces {}.
func TestRunContext_MissingConfig(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)

	// No config files — config.Load should fail gracefully

	hookInput := testHookInputKestrel
	output := captureContextOutput(t, hookInput)
	output = trimNewlines(output)

	// Must be valid JSON even when config is missing
	var v interface{}
	if err := json.Unmarshal([]byte(output), &v); err != nil {
		t.Fatalf("output is not valid JSON: %v\noutput: %q", err, output)
	}
	// Should output {} when config load fails
	if output != "{}" {
		t.Errorf("expected {} when config missing, got: %q", output)
	}
}

// TestExtractWorktreeHexID verifies worktree hex ID extraction from CWD paths.
func TestExtractWorktreeHexID(t *testing.T) {
	home, err := os.UserHomeDir()
	if err != nil {
		t.Skip("cannot determine home dir")
	}

	tests := []struct {
		name string
		cwd  string
		want string
	}{
		{
			name: "standard worktree path",
			cwd:  filepath.Join(home, ".ttal", "worktrees", "878619a0-ttal"),
			want: "878619a0",
		},
		{
			name: "multi-hyphen alias",
			cwd:  filepath.Join(home, ".ttal", "worktrees", "eb2fde5b-ttal-cli"),
			want: "eb2fde5b",
		},
		{
			name: "empty CWD",
			cwd:  "",
			want: "",
		},
		{
			name: "non-worktree path",
			cwd:  filepath.Join(home, "Code", "project"),
			want: "",
		},
		{
			name: "worktree subdir",
			cwd:  filepath.Join(home, ".ttal", "worktrees", "ec16980f-ttal", "cmd"),
			want: "ec16980f",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := extractWorktreeHexID(tt.cwd)
			if got != tt.want {
				t.Errorf("extractWorktreeHexID(%q) = %q, want %q", tt.cwd, got, tt.want)
			}
		})
	}
}

// TestRunContext_WorkerCWD_SetsJobID verifies that a worker session (CWD under ~/.ttal/worktrees/)
// has TTAL_JOB_ID derived from the worktree dir name and passed to $ cmd subprocesses.
func TestRunContext_WorkerCWD_SetsJobID(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)

	cfgDir := filepath.Join(tmp, ".config", "ttal")
	if err := os.MkdirAll(cfgDir, 0o755); err != nil {
		t.Fatalf("mkdir cfgDir: %v", err)
	}
	// Context template echoes TTAL_JOB_ID — set by extractWorktreeHexID from the CWD.
	promptsToml := "context = \"$ echo $TTAL_JOB_ID\"\n"
	if err := os.WriteFile(filepath.Join(cfgDir, "prompts.toml"), []byte(promptsToml), 0o644); err != nil {
		t.Fatalf("write prompts.toml: %v", err)
	}
	configToml := "[teams.default]\nteam_path = \"" + tmp + "\"\n"
	if err := os.WriteFile(filepath.Join(cfgDir, "config.toml"), []byte(configToml), 0o644); err != nil {
		t.Fatalf("write config.toml: %v", err)
	}

	// Create the worktree directory so the path is valid.
	worktreeCWD := filepath.Join(tmp, ".ttal", "worktrees", "ab12cd34-ttal")
	if err := os.MkdirAll(worktreeCWD, 0o755); err != nil {
		t.Fatalf("mkdir worktree: %v", err)
	}

	hookInput := `{"agent_type":"coder","cwd":"` + worktreeCWD + `"}`
	output := captureContextOutput(t, hookInput)
	output = trimNewlines(output)

	var resp ccHookResponse
	if err := json.Unmarshal([]byte(output), &resp); err != nil {
		t.Fatalf("output is not valid JSON: %v\noutput: %q", err, output)
	}
	if resp.HookSpecificOutput == nil {
		t.Fatalf("expected hookSpecificOutput, got: %q", output)
	}
	if !strings.Contains(resp.HookSpecificOutput.AdditionalContext, "ab12cd34") {
		t.Errorf("expected TTAL_JOB_ID 'ab12cd34' in additionalContext, got: %q",
			resp.HookSpecificOutput.AdditionalContext)
	}
}

// TestRunContext_NoContextKey verifies that a valid config without a 'context' prompt key
// produces {} (no context injection — non-agent sessions and unconfigured agents get no context).
func TestRunContext_NoContextKey(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)

	cfgDir := filepath.Join(tmp, ".config", "ttal")
	if err := os.MkdirAll(cfgDir, 0o755); err != nil {
		t.Fatalf("mkdir cfgDir: %v", err)
	}
	// prompts.toml exists but has no context key — only an unrelated key.
	promptsToml := "review = \"You are a reviewer.\"\n"
	if err := os.WriteFile(filepath.Join(cfgDir, "prompts.toml"), []byte(promptsToml), 0o644); err != nil {
		t.Fatalf("write prompts.toml: %v", err)
	}
	configToml := "[teams.default]\nteam_path = \"" + tmp + "\"\n"
	if err := os.WriteFile(filepath.Join(cfgDir, "config.toml"), []byte(configToml), 0o644); err != nil {
		t.Fatalf("write config.toml: %v", err)
	}

	output := captureContextOutput(t, testHookInputKestrel)
	output = trimNewlines(output)
	if output != "{}" {
		t.Errorf("expected {} when context key absent, got: %q", output)
	}
}

const testHookInputKestrel = `{"agent_type":"kestrel"}`

func trimNewlines(s string) string {
	for len(s) > 0 && (s[len(s)-1] == '\n' || s[len(s)-1] == '\r') {
		s = s[:len(s)-1]
	}
	return s
}
