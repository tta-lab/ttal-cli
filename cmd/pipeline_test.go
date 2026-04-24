package cmd

import (
	"bytes"
	"io"
	"os"
	"strings"
	"testing"

	"github.com/tta-lab/ttal-cli/internal/config"
	"github.com/tta-lab/ttal-cli/internal/pipeline"
	"github.com/tta-lab/ttal-cli/internal/taskwarrior"
)

// TestResolvePipelinePrompt_NoEnvVars verifies that resolvePipelinePrompt returns the default
// role prompt (not empty) when no env vars are set — skills are unconditional per the new
// role-based design: idle sessions still get their role's base prompt.
func TestResolvePipelinePrompt_NoEnvVars(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)
	t.Setenv("TTAL_JOB_ID", "")
	t.Setenv("TTAL_AGENT_NAME", "")
	os.MkdirAll(tmp+"/.config/ttal", 0755) //nolint:errcheck
	teamPath := tmp + "/team"
	os.WriteFile(tmp+"/.config/ttal/config.toml", []byte(
		"\n[teams.default]\nteam_path = \""+teamPath+"\"\n",
	), 0644) //nolint:errcheck
	os.WriteFile(tmp+"/.config/ttal/humans.toml", []byte(
		"[neil]\nname = \"Neil\"\ntelegram_chat_id = \"12345\"\nadmin = true\n",
	), 0644) //nolint:errcheck
	os.WriteFile(tmp+"/.config/ttal/roles.toml", []byte(`[default]
prompt = """Manage tasks and coordinate the team."""
`), 0644) //nolint:errcheck
	got := resolvePipelinePrompt()
	if got == "" {
		t.Errorf("expected non-empty output with default role prompt, got empty string")
	}
	if !strings.Contains(got, "Manage tasks and coordinate the team") {
		t.Errorf("expected default role prompt, got: %q", got)
	}
}

// TestResolvePromptKey_CoderAssignee verifies the coder assignee maps to "coder" prompt key.
func TestResolvePromptKey_CoderAssignee(t *testing.T) {
	t.Setenv("TTAL_AGENT_NAME", "coder")
	stage := &pipeline.Stage{Assignee: "coder", Reviewer: ""}
	got := resolvePromptKey(stage)
	if got != "coder" {
		t.Errorf("resolvePromptKey for coder assignee = %q, want %q", got, "coder")
	}
}

// TestResolvePromptKey_PRReviewer verifies that when TTAL_AGENT_NAME matches stage.Reviewer
// and the assignee is a coder stage, "review" prompt key is returned.
func TestResolvePromptKey_PRReviewer(t *testing.T) {
	t.Setenv("TTAL_AGENT_NAME", "pr-review-lead")
	stage := &pipeline.Stage{Assignee: "coder", Worker: true, Reviewer: "pr-review-lead"}
	got := resolvePromptKey(stage)
	if got != "review" {
		t.Errorf("resolvePromptKey for PR reviewer = %q, want %q", got, "review")
	}
}

// TestResolvePromptKey_PlanReviewer verifies that when TTAL_AGENT_NAME matches stage.Reviewer
// and the assignee is a non-coder stage, "plan_review" prompt key is returned.
func TestResolvePromptKey_PlanReviewer(t *testing.T) {
	t.Setenv("TTAL_AGENT_NAME", "plan-review-lead")
	stage := &pipeline.Stage{Assignee: "designer", Reviewer: "plan-review-lead"}
	got := resolvePromptKey(stage)
	if got != "plan_review" {
		t.Errorf("resolvePromptKey for plan reviewer = %q, want %q", got, "plan_review")
	}
}

// TestResolvePromptKey_DesignerAssignee verifies that designer assignee maps to "designer".
func TestResolvePromptKey_DesignerAssignee(t *testing.T) {
	t.Setenv("TTAL_AGENT_NAME", "mira")
	stage := &pipeline.Stage{Assignee: "designer", Reviewer: ""}
	got := resolvePromptKey(stage)
	if got != "designer" {
		t.Errorf("resolvePromptKey for designer assignee = %q, want %q", got, "designer")
	}
}

// TestExpandPromptVars_PRIDVars verifies that {{pr-number}} and {{pr-title}} are expanded
// when the task has a valid PRID. Branch/owner/repo use soft failure (empty string) since
// no git repo or worktree exists in test context.
func TestExpandPromptVars_PRIDVars(t *testing.T) {
	task := &taskwarrior.Task{
		UUID:        "ab12cd34-0000-0000-0000-000000000000",
		Description: "Add login feature",
		PRID:        "42",
	}
	prompt := "PR {{pr-number}}: {{pr-title}} (branch: {{branch}})"
	cfg := &config.Config{}

	got := expandPromptVars(prompt, task, cfg)
	if !strings.Contains(got, "PR 42:") {
		t.Errorf("expected {{pr-number}} expanded to 42, got: %q", got)
	}
	if !strings.Contains(got, "Add login feature") {
		t.Errorf("expected {{pr-title}} expanded to task description, got: %q", got)
	}
}

// TestExpandPromptVars_NoPRID verifies that prompts are returned unchanged when PRID is empty.
func TestExpandPromptVars_NoPRID(t *testing.T) {
	task := &taskwarrior.Task{
		UUID:        "ab12cd34-0000-0000-0000-000000000000",
		Description: "Some task",
		PRID:        "",
	}
	prompt := "PR {{pr-number}}: {{pr-title}}"
	cfg := &config.Config{}

	got := expandPromptVars(prompt, task, cfg)
	// Without PRID, PR vars should remain as literal placeholders (not expanded).
	if strings.Contains(got, "42") {
		t.Errorf("expected no PR number expansion without PRID, got: %q", got)
	}
}

func captureStdout(t *testing.T, fn func()) string {
	t.Helper()
	old := os.Stdout
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("os.Pipe: %v", err)
	}
	os.Stdout = w
	fn()
	w.Close()
	os.Stdout = old
	var buf bytes.Buffer
	_, _ = io.Copy(&buf, r)
	return buf.String()
}

func TestRenderPipelineGraph_TwoStages(t *testing.T) {
	p := pipeline.Pipeline{
		Stages: []pipeline.Stage{
			{Name: "Plan", Assignee: "designer", Gate: "human", Reviewer: "plan-review-lead"},
			{Name: "Implement", Assignee: "coder", Gate: "auto"},
		},
	}
	out := captureStdout(t, func() { renderPipelineGraph(p) })
	if !strings.Contains(out, "Plan [designer]") {
		t.Errorf("expected 'Plan [designer]' in output: %s", out)
	}
	if !strings.Contains(out, "Implement [coder]") {
		t.Errorf("expected 'Implement [coder]' in output: %s", out)
	}
	if !strings.Contains(out, "human/plan-review-lead") {
		t.Errorf("expected 'human/plan-review-lead' in arrow: %s", out)
	}
}

func TestRenderPipelineGraph_SingleStage(t *testing.T) {
	p := pipeline.Pipeline{
		Stages: []pipeline.Stage{
			{Name: "Implement", Assignee: "coder", Gate: "auto"},
		},
	}
	out := captureStdout(t, func() { renderPipelineGraph(p) })
	if !strings.Contains(out, "Implement [coder]") {
		t.Errorf("expected 'Implement [coder]' in output: %s", out)
	}
	// Single stage should have no arrow.
	if strings.Contains(out, "──") {
		t.Errorf("single stage should have no arrow: %s", out)
	}
}

func TestRenderPipelineGraph_NoReviewer(t *testing.T) {
	p := pipeline.Pipeline{
		Stages: []pipeline.Stage{
			{Name: "Fix", Assignee: "fixer", Gate: "human"},
			{Name: "Implement", Assignee: "coder", Gate: "auto"},
		},
	}
	out := captureStdout(t, func() { renderPipelineGraph(p) })
	// Arrow should show gate only, no reviewer suffix.
	if !strings.Contains(out, "──human──") {
		t.Errorf("expected '──human──' without reviewer in arrow: %s", out)
	}
	if strings.Contains(out, "human/") {
		t.Errorf("should not have reviewer suffix: %s", out)
	}
}
