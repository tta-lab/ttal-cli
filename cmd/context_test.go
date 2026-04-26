package cmd

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/spf13/cobra"
)

const testHumansToml = "[neil]\nname = \"Neil\"\ntelegram_chat_id = \"12345\"\nadmin = true\n"

// TestContext_ManagerTemplate verifies that runContext emits the manager template
// (contains § Diary, § Pairing, § Role, § Task) for a manager agent.
func TestContext_ManagerTemplate(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)
	t.Setenv("TTAL_AGENT_NAME", "kestrel")
	t.Setenv("TTAL_JOB_ID", "")

	cfgDir := filepath.Join(tmp, ".config", "ttal")
	_ = os.MkdirAll(cfgDir, 0o755) //nolint:errcheck

	promptsToml := "context_manager = \"§ Diary\\n$ echo diary\\n§ Pairing\\n" +
		"$ echo pairing\\n§ Role\\n$ echo role\\n§ Task\\n$ echo task\\n\""
	_ = os.WriteFile(filepath.Join(cfgDir, "prompts.toml"), []byte(promptsToml), 0o644) //nolint:errcheck

	configToml := "[teams.default]\nteam_path = \"" + tmp + "\"\n"
	_ = os.WriteFile(filepath.Join(cfgDir, "config.toml"), []byte(configToml), 0o644) //nolint:errcheck

	humansToml := testHumansToml
	_ = os.WriteFile(filepath.Join(cfgDir, "humans.toml"), []byte(humansToml), 0o644) //nolint:errcheck

	// Create kestrel agent dir so agentfs.HasAgent returns true.
	kestrelDir := filepath.Join(tmp, "kestrel")
	_ = os.MkdirAll(kestrelDir, 0o755) //nolint:errcheck
	agentsMD := "---\nrole: designer\n---\n"
	_ = os.WriteFile(filepath.Join(kestrelDir, "AGENTS.md"), []byte(agentsMD), 0o644) //nolint:errcheck

	var got string
	contextCmd.SetOut(&bytes.Buffer{})
	old := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w
	_ = runContext(&cobra.Command{}, []string{})
	w.Close()
	os.Stdout = old
	buf := &bytes.Buffer{}
	_, _ = buf.ReadFrom(r)
	got = buf.String()

	if !strings.Contains(got, "§ Diary") {
		t.Errorf("expected § Diary in output, got: %q", got)
	}
	if !strings.Contains(got, "§ Pairing") {
		t.Errorf("expected § Pairing in output, got: %q", got)
	}
	if !strings.Contains(got, "§ Role") {
		t.Errorf("expected § Role in output, got: %q", got)
	}
	if !strings.Contains(got, "§ Task") {
		t.Errorf("expected § Task in output, got: %q", got)
	}
}

// TestContext_WorkerTemplate verifies that runContext emits the worker template
// (contains § Pairing, § Role, § Task but NOT § Diary) for a non-manager agent.
func TestContext_WorkerTemplate(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)
	t.Setenv("TTAL_AGENT_NAME", "coder")
	t.Setenv("TTAL_JOB_ID", "227b31a9")

	cfgDir := filepath.Join(tmp, ".config", "ttal")
	_ = os.MkdirAll(cfgDir, 0o755) //nolint:errcheck

	promptsToml := "context_worker = \"§ Pairing\\n$ echo pairing\\n§ Role\\n$ echo role\\n§ Task\\n$ echo task\\n\""
	_ = os.WriteFile(filepath.Join(cfgDir, "prompts.toml"), []byte(promptsToml), 0o644) //nolint:errcheck

	configToml := "[teams.default]\nteam_path = \"" + tmp + "\"\n"
	_ = os.WriteFile(filepath.Join(cfgDir, "config.toml"), []byte(configToml), 0o644) //nolint:errcheck

	humansToml := testHumansToml
	_ = os.WriteFile(filepath.Join(cfgDir, "humans.toml"), []byte(humansToml), 0o644) //nolint:errcheck

	// Do NOT create coder/ AGENTS.md — coder is not a manager.

	var got string
	old := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w
	_ = runContext(&cobra.Command{}, []string{})
	w.Close()
	os.Stdout = old
	buf := &bytes.Buffer{}
	_, _ = buf.ReadFrom(r)
	got = buf.String()

	if !strings.Contains(got, "§ Pairing") {
		t.Errorf("expected § Pairing in output, got: %q", got)
	}
	if !strings.Contains(got, "§ Role") {
		t.Errorf("expected § Role in output, got: %q", got)
	}
	if !strings.Contains(got, "§ Task") {
		t.Errorf("expected § Task in output, got: %q", got)
	}
	if strings.Contains(got, "§ Diary") {
		t.Errorf("expected no § Diary in worker output, got: %q", got)
	}
}

// TestContext_NoAgentNoOutput verifies that runContext outputs nothing when
// TTAL_AGENT_NAME is unset (non-agent session).
func TestContext_NoAgentNoOutput(t *testing.T) {
	t.Setenv("TTAL_AGENT_NAME", "")
	t.Setenv("TTAL_JOB_ID", "")

	old := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w
	err := runContext(&cobra.Command{}, []string{})
	w.Close()
	os.Stdout = old
	buf := &bytes.Buffer{}
	_, _ = buf.ReadFrom(r)
	got := buf.String()

	if err != nil {
		t.Errorf("expected nil error, got: %v", err)
	}
	if got != "" {
		t.Errorf("expected empty output for non-agent session, got: %q", got)
	}
}

// TestContext_NoTemplate verifies that runContext outputs nothing when neither
// context_manager nor context_worker is configured.
func TestContext_NoTemplate(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)
	t.Setenv("TTAL_AGENT_NAME", "kestrel")
	t.Setenv("TTAL_JOB_ID", "")

	cfgDir := filepath.Join(tmp, ".config", "ttal")
	_ = os.MkdirAll(cfgDir, 0o755) //nolint:errcheck

	promptsToml := "review = \"You are a reviewer.\"\n"
	_ = os.WriteFile(filepath.Join(cfgDir, "prompts.toml"), []byte(promptsToml), 0o644) //nolint:errcheck

	configToml := "[teams.default]\nteam_path = \"" + tmp + "\"\n"
	_ = os.WriteFile(filepath.Join(cfgDir, "config.toml"), []byte(configToml), 0o644) //nolint:errcheck

	humansToml := testHumansToml
	_ = os.WriteFile(filepath.Join(cfgDir, "humans.toml"), []byte(humansToml), 0o644) //nolint:errcheck

	kestrelDir := filepath.Join(tmp, "kestrel")
	_ = os.MkdirAll(kestrelDir, 0o755)                                                                    //nolint:errcheck
	_ = os.WriteFile(filepath.Join(kestrelDir, "AGENTS.md"), []byte("---\nrole: designer\n---\n"), 0o644) //nolint:errcheck

	old := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w
	err := runContext(&cobra.Command{}, []string{})
	w.Close()
	os.Stdout = old
	buf := &bytes.Buffer{}
	_, _ = buf.ReadFrom(r)
	got := buf.String()

	if err != nil {
		t.Errorf("expected nil error, got: %v", err)
	}
	if got != "" {
		t.Errorf("expected empty output with no context template, got: %q", got)
	}
}

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
		{name: "standard worktree path", cwd: filepath.Join(home, ".ttal", "worktrees", "878619a0-ttal"), want: "878619a0"},
		{name: "multi-hyphen alias", cwd: filepath.Join(home, ".ttal", "worktrees", "eb2fde5b-ttal-cli"), want: "eb2fde5b"},
		{name: "empty CWD", cwd: "", want: ""},
		{name: "non-worktree path", cwd: filepath.Join(home, "Code", "project"), want: ""},
		{name: "worktree subdir", cwd: filepath.Join(home, ".ttal", "worktrees", "ec16980f-ttal", "cmd"), want: "ec16980f"},
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
