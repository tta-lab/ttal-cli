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
	configToml := "[teams.default]\nteam_path = \"" + tmp + "\"\nchat_id = \"12345\"\n"
	if err := os.WriteFile(filepath.Join(cfgDir, "config.toml"), []byte(configToml), 0o644); err != nil {
		t.Fatalf("write config.toml: %v", err)
	}

	// Create kestrel agent dir so agentfs.HasAgent passes (kestrel is a manager).
	kestrelDir := filepath.Join(tmp, "kestrel")
	if err := os.MkdirAll(kestrelDir, 0o755); err != nil {
		t.Fatalf("mkdir kestrel: %v", err)
	}
	if err := os.WriteFile(filepath.Join(kestrelDir, "AGENTS.md"), []byte("---\nrole: fixer\n---\n"), 0o644); err != nil {
		t.Fatalf("write AGENTS.md: %v", err)
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

// TestRunContext_NonManagerAgent_EmitsNoop verifies that a non-manager agent
// (no AGENTS.md under team_path) receives a no-op {} response.
func TestRunContext_NonManagerAgent_EmitsNoop(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)

	cfgDir := filepath.Join(tmp, ".config", "ttal")
	if err := os.MkdirAll(cfgDir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	promptsToml := "context = \"$ echo context-from-hook\"\n"
	if err := os.WriteFile(filepath.Join(cfgDir, "prompts.toml"), []byte(promptsToml), 0o644); err != nil {
		t.Fatalf("write prompts.toml: %v", err)
	}
	configToml := "[teams.default]\nteam_path = \"" + tmp + "\"\nchat_id = \"12345\"\n"
	if err := os.WriteFile(filepath.Join(cfgDir, "config.toml"), []byte(configToml), 0o644); err != nil {
		t.Fatalf("write config.toml: %v", err)
	}

	// No manager agent directories created — "coder" has no AGENTS.md, so
	// agentfs.HasAgent returns false and runContext emits {}.
	hookInput := `{"agent_type":"coder"}`
	output := captureContextOutput(t, hookInput)
	output = trimNewlines(output)
	if output != "{}" {
		t.Errorf("expected {} for non-manager agent, got: %q", output)
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
	configToml := "[teams.default]\nteam_path = \"" + tmp + "\"\nchat_id = \"12345\"\n"
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

// TestContext_SmokeTest_RoleBasedSkills verifies that resolvePipelinePrompt returns
// role-based skills for an idle agent session (no active task).
//
// kestrel is role=fixer; fixer gets default_skills + ["sp-debugging"].
// The fixer role prompt includes a fetch-on-demand instruction; skill bodies are NOT
// inlined (exceeds CC hook size budget). The test verifies no skill headers appear
// and the fetch instruction is present.
//
//nolint:gocyclo
func TestContext_SmokeTest_RoleBasedSkills(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)

	cfgDir := filepath.Join(tmp, ".config", "ttal")
	if err := os.MkdirAll(cfgDir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}

	// Write roles.toml: fixer gets sp-debugging extra skill with fetch-on-demand instruction.
	rolesToml := "\ndefault_skills = [\"task-tree\", \"flicknote\", \"ei-ask\"]\n\n[fixer]\n" +
		"prompt = \"\"\"Execute `skill get sp-debugging` to load the " +
		"bug-fix-design methodology.\n\nDiagnose this bug and write a fix plan.\"\"\"\n" +
		"extra_skills = [\"sp-debugging\"]\n"
	if err := os.WriteFile(filepath.Join(cfgDir, "roles.toml"), []byte(rolesToml), 0o644); err != nil {
		t.Fatalf("write roles.toml: %v", err)
	}

	// Write prompts.toml (required by config.Load).
	promptsToml := "review = \"You are a reviewer.\"\n"
	if err := os.WriteFile(filepath.Join(cfgDir, "prompts.toml"), []byte(promptsToml), 0o644); err != nil {
		t.Fatalf("write prompts.toml: %v", err)
	}

	// Write config.toml.
	configToml := "[teams.default]\nteam_path = \"" + tmp + "\"\nchat_id = \"12345\"\n"
	if err := os.WriteFile(filepath.Join(cfgDir, "config.toml"), []byte(configToml), 0o644); err != nil {
		t.Fatalf("write config.toml: %v", err)
	}

	// Create kestrel agent dir so agentfs.RoleOf succeeds.
	kestrelDir := filepath.Join(tmp, "kestrel")
	if err := os.MkdirAll(kestrelDir, 0o755); err != nil {
		t.Fatalf("mkdir kestrel: %v", err)
	}
	if err := os.WriteFile(filepath.Join(kestrelDir, "AGENTS.md"), []byte("---\nrole: fixer\n---\n"), 0o644); err != nil {
		t.Fatalf("write AGENTS.md: %v", err)
	}

	// Create pipelines.toml (required by resolvePipelinePrompt pipeline path).
	if err := os.WriteFile(filepath.Join(cfgDir, "pipelines.toml"), []byte(`
[bugfix]
description = "Bug fix"
tags = ["bugfix"]

[[bugfix.stages]]
name = "Fix"
assignee = "fixer"
gate = "human"
`), 0o644); err != nil {
		t.Fatalf("write pipelines.toml: %v", err)
	}

	// Set TTAL_AGENT_NAME so resolvePipelinePrompt uses agent-first path.
	// Unset TTAL_JOB_ID so resolveCurrentTaskForPrompt returns nil (idle session).
	t.Setenv("TTAL_AGENT_NAME", "kestrel")
	t.Setenv("TTAL_JOB_ID", "")

	got := resolvePipelinePrompt()
	if got == "" {
		t.Fatalf("resolvePipelinePrompt() returned empty for idle fixer session")
	}

	// Skill bodies must NOT appear (not inlined — exceeds CC hook size budget).
	if strings.Contains(got, "# sp-debugging [skill]") {
		t.Errorf("expected no '# sp-debugging [skill]' in output, got: %q", got)
	}
	if strings.Contains(got, "# task-tree [skill]") {
		t.Errorf("expected no '# task-tree [skill]' in output, got: %q", got)
	}
	if strings.Contains(got, "[skill]") {
		t.Errorf("expected no '[skill]' marker in output, got: %q", got)
	}
	// Fetch-on-demand instruction must appear.
	if !strings.Contains(got, "skill get sp-debugging") {
		t.Errorf("expected 'skill get sp-debugging' in output, got: %q", got)
	}
	// Role prompt must appear.
	if !strings.Contains(got, "Diagnose this bug") {
		t.Errorf("expected fixer role prompt in output, got: %q", got)
	}
	// No task section for idle session.
	if strings.Contains(got, "## Task") {
		t.Errorf("expected no '## Task' section for idle session, got: %q", got)
	}
}

// TestContext_SmokeTest_ActiveTaskPipeline verifies that resolvePipelinePrompt
// exercises the active-task branch (task != nil) and returns a combined
// ## Task + role prompt output with no skill bodies inlined.
func TestContext_SmokeTest_ActiveTaskPipeline(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)

	cfgDir := filepath.Join(tmp, ".config", "ttal")
	if err := os.MkdirAll(cfgDir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}

	rolesToml := "\ndefault_skills = [\"task-tree\", \"flicknote\", \"ei-ask\"]\n\n[fixer]\n" +
		"prompt = \"\"\"Execute `skill get sp-debugging` to load the bug-fix-design " +
		"methodology.\n\nDiagnose this bug and write a fix plan.\"\"\"\n" +
		"extra_skills = [\"sp-debugging\"]\n"
	if err := os.WriteFile(filepath.Join(cfgDir, "roles.toml"), []byte(rolesToml), 0o644); err != nil {
		t.Fatalf("write roles.toml: %v", err)
	}

	promptsToml := `
default = "Manage tasks."
fixer = "Diagnose this bug and write a fix plan."
`
	if err := os.WriteFile(filepath.Join(cfgDir, "prompts.toml"), []byte(promptsToml), 0o644); err != nil {
		t.Fatalf("write prompts.toml: %v", err)
	}

	configToml := "[teams.default]\nteam_path = \"" + tmp + "\"\nchat_id = \"12345\"\n"
	if err := os.WriteFile(filepath.Join(cfgDir, "config.toml"), []byte(configToml), 0o644); err != nil {
		t.Fatalf("write config.toml: %v", err)
	}

	kestrelDir := filepath.Join(tmp, "kestrel")
	if err := os.MkdirAll(kestrelDir, 0o755); err != nil {
		t.Fatalf("mkdir kestrel: %v", err)
	}
	if err := os.WriteFile(filepath.Join(kestrelDir, "AGENTS.md"), []byte("---\nrole: fixer\n---\n"), 0o644); err != nil {
		t.Fatalf("write AGENTS.md: %v", err)
	}

	pipelinesToml := `
[bugfix]
description = "Bug fix pipeline"
tags = ["bugfix"]

[[bugfix.stages]]
name = "Fix"
assignee = "fixer"
gate = "human"
`
	if err := os.WriteFile(filepath.Join(cfgDir, "pipelines.toml"), []byte(pipelinesToml), 0o644); err != nil {
		t.Fatalf("write pipelines.toml: %v", err)
	}

	// Write a fake task UUID so resolveCurrentTaskForPrompt hits the TTAL_JOB_ID path.
	t.Setenv("TTAL_AGENT_NAME", "kestrel")
	t.Setenv("TTAL_JOB_ID", "00000000-0000-0000-0000-000000000001")

	got := resolvePipelinePrompt()

	// Active task path is taken even when task export fails (logged, returns nil-ish behavior).
	// The combined output should not contain skill bodies.
	if strings.Contains(got, "[skill]") {
		t.Errorf("expected no '[skill]' marker in active-task output, got: %q", got)
	}
}

const testHookInputKestrel = `{"agent_type":"kestrel"}`

func trimNewlines(s string) string {
	for len(s) > 0 && (s[len(s)-1] == '\n' || s[len(s)-1] == '\r') {
		s = s[:len(s)-1]
	}
	return s
}
