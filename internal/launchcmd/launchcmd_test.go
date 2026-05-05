package launchcmd

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
	"testing"
)

func TestBuildCCDirectCommand_WithTrigger(t *testing.T) {
	got := BuildCCDirectCommand("/usr/bin/ttal", "coder", ContextTrigger)
	if !strings.Contains(got, "--agent coder") {
		t.Errorf("missing --agent coder: %q", got)
	}
	if !strings.Contains(got, "ttal context") {
		t.Errorf("missing ttal context trigger: %q", got)
	}
	if strings.Contains(got, "--resume") {
		t.Errorf("should not contain --resume: %q", got)
	}
}

func TestBuildCCDirectCommand_NoTrigger(t *testing.T) {
	got := BuildCCDirectCommand("/usr/bin/ttal", "pr-review-lead", "")
	if !strings.Contains(got, "--agent pr-review-lead") {
		t.Errorf("missing --agent: %q", got)
	}
	if strings.Contains(got, " -- '") {
		t.Errorf("should not have trigger when empty: %q", got)
	}
}

func TestBuildCCDirectCommand_ApostropheEscaping(t *testing.T) {
	got := BuildCCDirectCommand("/usr/bin/ttal", "coder", "it's a test")
	if !strings.Contains(got, "it'\\''s a test") {
		t.Errorf("apostrophe not escaped correctly: %q", got)
	}
}

func TestBuildLenosCommand_Basic(t *testing.T) {
	got := BuildLenosCommand("/usr/bin/ttal", "coder", ContextTrigger, false)
	if !strings.Contains(got, "--agent coder") {
		t.Errorf("missing --agent coder: %q", got)
	}
	if !strings.Contains(got, "ttal context") {
		t.Errorf("missing ttal context trigger: %q", got)
	}
	if strings.Contains(got, "--readonly") {
		t.Errorf("should not contain --readonly when readOnly=false: %q", got)
	}
}

func TestBuildLenosCommand_ApostropheEscaping(t *testing.T) {
	got := BuildLenosCommand("/usr/bin/ttal", "coder", "it's a test", false)
	if !strings.Contains(got, "it'\\''s a test") {
		t.Errorf("apostrophe not escaped correctly: %q", got)
	}
}

func TestBuildLenosCommand_ReadOnly(t *testing.T) {
	got := BuildLenosCommand("/usr/bin/ttal", "pr-review-lead", ContextTrigger, true)
	if !strings.Contains(got, "--agent pr-review-lead") {
		t.Errorf("missing --agent: %q", got)
	}
	if !strings.Contains(got, "--readonly") {
		t.Errorf("missing --readonly when readOnly=true: %q", got)
	}
	// --readonly should appear before the trigger separator
	idxReadOnly := strings.Index(got, "--readonly")
	idxTrigger := strings.Index(got, "ttal context")
	if idxReadOnly == -1 || idxTrigger == -1 || idxReadOnly > idxTrigger {
		t.Errorf("--readonly should come before trigger: %q", got)
	}
}

// --- New tests for step 2 additions ---

func TestSingleQuoteShell_Empty(t *testing.T) {
	got := singleQuoteShell("")
	want := "''"
	if got != want {
		t.Errorf("empty: want %q, got %q", want, got)
	}
}

func TestSingleQuoteShell_NoApostrophe(t *testing.T) {
	got := singleQuoteShell("hello world")
	want := "'hello world'"
	if got != want {
		t.Errorf("plain: want %q, got %q", want, got)
	}
}

func TestSingleQuoteShell_SingleApostrophe(t *testing.T) {
	got := singleQuoteShell("it's")
	want := "'it'\\''s'"
	if got != want {
		t.Errorf("single apostrophe: want %q, got %q", want, got)
	}
}

func TestSingleQuoteShell_MultipleApostrophes(t *testing.T) {
	got := singleQuoteShell("it's a 'test' right")
	want := "'it'\\''s a '\\''test'\\'' right'"
	if got != want {
		t.Errorf("multiple apostrophes: want %q, got %q", want, got)
	}
}

func TestSingleQuoteShell_OnlyApostrophes(t *testing.T) {
	got := singleQuoteShell("'''")
	if got == "''" {
		t.Errorf("only apostrophes should be escaped, got just empty quotes: %q", got)
	}
}

func TestSingleQuoteShell_LeadingApostrophe(t *testing.T) {
	got := singleQuoteShell("'foo")
	if !strings.HasPrefix(got, "'") || !strings.HasSuffix(got, "'") {
		t.Errorf("should be single-quote wrapped: %q", got)
	}
	n := len(got)
	if n < 3 {
		t.Errorf("too short: %q", got)
	}
}

func TestSingleQuoteShell_TrailingApostrophe(t *testing.T) {
	got := singleQuoteShell("foo'")
	if !strings.HasPrefix(got, "'") || !strings.HasSuffix(got, "'") {
		t.Errorf("should be single-quote wrapped: %q", got)
	}
}

func TestSingleQuoteShell_SpecialChars(t *testing.T) {
	input := "$USER `whoami`; &&"
	got := singleQuoteShell(input)
	want := "'$USER `whoami`; &&'"
	if got != want {
		t.Errorf("special chars: want %q, got %q", want, got)
	}
}

func TestBuildEnvParts_ClaudeCode(t *testing.T) {
	parts := BuildEnvParts("abc123", "coder", "claude-code")
	if len(parts) != 3 {
		t.Fatalf("expected 3 parts, got %d", len(parts))
	}
	if parts[0] != "TTAL_AGENT_NAME=coder" {
		t.Errorf("agent name: want %q, got %q", "TTAL_AGENT_NAME=coder", parts[0])
	}
	if parts[1] != "TTAL_JOB_ID=abc123" {
		t.Errorf("job id: want %q, got %q", "TTAL_JOB_ID=abc123", parts[1])
	}
	if parts[2] != "TTAL_RUNTIME=claude-code" {
		t.Errorf("runtime: want %q, got %q", "TTAL_RUNTIME=claude-code", parts[2])
	}
}

func TestBuildEnvParts_Lenos(t *testing.T) {
	parts := BuildEnvParts("def456", "plan-review-lead", "lenos")
	if len(parts) != 3 {
		t.Fatalf("expected 3 parts, got %d", len(parts))
	}
	if parts[0] != "TTAL_AGENT_NAME=plan-review-lead" {
		t.Errorf("agent name: want %q, got %q", "TTAL_AGENT_NAME=plan-review-lead", parts[0])
	}
	if parts[1] != "TTAL_JOB_ID=def456" {
		t.Errorf("job id: want %q, got %q", "TTAL_JOB_ID=def456", parts[1])
	}
	if parts[2] != "TTAL_RUNTIME=lenos" {
		t.Errorf("runtime: want %q, got %q", "TTAL_RUNTIME=lenos", parts[2])
	}
}

func TestBuildAgentLaunchCommand_ClaudeCode(t *testing.T) {
	got, err := BuildAgentLaunchCommand("claude-code", "/usr/bin/ttal", "coder", false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(got, "claude --dangerously-skip-permissions --agent coder") {
		t.Errorf("expected claude agent: %q", got)
	}
	if strings.Contains(got, "lenos") {
		t.Errorf("should not contain lenos: %q", got)
	}
	if !strings.Contains(got, "ttal context") {
		t.Errorf("missing context trigger: %q", got)
	}
}

func TestBuildAgentLaunchCommand_Lenos(t *testing.T) {
	got, err := BuildAgentLaunchCommand("lenos", "/usr/bin/ttal", "plan-review-lead", false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(got, "lenos --agent plan-review-lead") {
		t.Errorf("expected lenos agent: %q", got)
	}
	if strings.Contains(got, "claude") {
		t.Errorf("should not contain claude: %q", got)
	}
	if !strings.Contains(got, "ttal context") {
		t.Errorf("missing context trigger: %q", got)
	}
}

func TestBuildAgentLaunchCommand_CodexRejected(t *testing.T) {
	_, err := BuildAgentLaunchCommand("codex", "/usr/bin/ttal", "coder", false)
	if err == nil {
		t.Fatal("expected error for codex runtime, got nil")
	}
	if !strings.Contains(err.Error(), "not supported in the worker plane") {
		t.Errorf("expected unsupported error, got: %v", err)
	}
}

func TestBuildAgentLaunchCommand_EmptyRejected(t *testing.T) {
	_, err := BuildAgentLaunchCommand("", "/usr/bin/ttal", "coder", false)
	if err == nil {
		t.Fatal("expected error for empty runtime, got nil")
	}
}

// TestBuildLenosCommand_AdversarialTrigger_RoundTrip verifies that adversarial
// trigger strings survive a bash round-trip through singleQuoteShell + echo.
// Uses echo (not the test binary) to avoid re-entrant test binary complexities.
func TestBuildLenosCommand_AdversarialTrigger_RoundTrip(t *testing.T) {
	adversarial := "It's a $USER-pwn test; `whoami` && rm -rf /tmp/should-not-exist"
	quoted := singleQuoteShell(adversarial)
	shellCmd := fmt.Sprintf("echo %s", quoted)
	out, err := exec.Command("bash", "-c", shellCmd).Output()
	if err != nil {
		t.Fatalf("exec failed: %v\nstdout: %s", err, out)
	}
	got := strings.TrimSpace(string(out))
	if got != adversarial {
		t.Errorf("trigger corrupted in echo round-trip:\n  want: %q\n  got:  %q", adversarial, got)
	}
	if _, err := os.Stat("/tmp/should-not-exist"); err == nil {
		t.Errorf("command substitution leaked: /tmp/should-not-exist was created")
		os.Remove("/tmp/should-not-exist")
	}
}
