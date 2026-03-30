package promptrender

import (
	"strings"
	"testing"
)

func TestRenderTemplate_PlainTextOnly(t *testing.T) {
	tmpl := "Hello\nWorld"
	got := RenderTemplate(tmpl, "agent", "team", nil)
	if !strings.Contains(got, "Hello") || !strings.Contains(got, "World") {
		t.Errorf("plain text not passed through: %q", got)
	}
}

func TestRenderTemplate_CommandSuccess(t *testing.T) {
	tmpl := "$ echo hello"
	got := RenderTemplate(tmpl, "agent", "team", nil)
	want := "--- echo hello ---\nhello"
	if !strings.Contains(got, want) {
		t.Errorf("expected header+output %q in result %q", want, got)
	}
}

func TestRenderTemplate_CommandHeaderFormat(t *testing.T) {
	tmpl := "$ echo test"
	got := RenderTemplate(tmpl, "agent", "team", nil)
	// Assert exact header format: --- <cmd> ---\n<output>\n\n
	if !strings.HasPrefix(got, "--- echo test ---\ntest") {
		t.Errorf("wrong header format: %q", got)
	}
}

func TestRenderTemplate_CommandFailure(t *testing.T) {
	tmpl := "before\n$ false\nafter"
	got := RenderTemplate(tmpl, "agent", "team", nil)
	if strings.Contains(got, "false") {
		t.Errorf("failed command output should be skipped: %q", got)
	}
	if !strings.Contains(got, "before") {
		t.Errorf("text before failed command should still appear: %q", got)
	}
	if !strings.Contains(got, "after") {
		t.Errorf("text after failed command should still appear: %q", got)
	}
}

func TestRenderTemplate_Mixed(t *testing.T) {
	tmpl := "intro\n$ echo hello\noutro"
	got := RenderTemplate(tmpl, "agent", "team", nil)
	if !strings.Contains(got, "intro") {
		t.Errorf("intro text missing: %q", got)
	}
	if !strings.Contains(got, "--- echo hello ---\nhello") {
		t.Errorf("command output missing: %q", got)
	}
	if !strings.Contains(got, "outro") {
		t.Errorf("outro text missing: %q", got)
	}
}

func TestRenderTemplate_TemplateVarExpansion(t *testing.T) {
	tmpl := "$ echo {{agent-name}}-{{team-name}}"
	got := RenderTemplate(tmpl, "myagent", "myteam", nil)
	if !strings.Contains(got, "myagent-myteam") {
		t.Errorf("template vars not expanded before command execution: %q", got)
	}
}

func TestRenderTemplate_EmptyCommandOutput(t *testing.T) {
	// Command succeeds but produces no output — should be skipped entirely.
	tmpl := "before\n$ true\nafter"
	got := RenderTemplate(tmpl, "agent", "team", nil)
	if strings.Contains(got, "--- true ---") {
		t.Errorf("empty command output should produce no header: %q", got)
	}
	if !strings.Contains(got, "before") || !strings.Contains(got, "after") {
		t.Errorf("surrounding text missing: %q", got)
	}
}

func TestRenderTemplate_EnvPassedToSubprocess(t *testing.T) {
	tmpl := "$ echo $TTAL_TEST_VAR"
	env := []string{"TTAL_TEST_VAR=hello_from_env"}
	got := RenderTemplate(tmpl, "agent", "team", env)
	if !strings.Contains(got, "hello_from_env") {
		t.Errorf("env var not passed to subprocess: %q", got)
	}
}
