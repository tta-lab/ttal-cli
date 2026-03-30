package daemon

import (
	"strings"
	"testing"
)

func TestEvaluateBreatheContext_EmptyCommands(t *testing.T) {
	if got := evaluateBreatheContext(nil, "yuki", "guion"); got != "" {
		t.Errorf("expected empty string for nil commands, got %q", got)
	}
	if got := evaluateBreatheContext([]string{}, "yuki", "guion"); got != "" {
		t.Errorf("expected empty string for empty commands, got %q", got)
	}
}

func TestEvaluateBreatheContext_TemplateExpansion(t *testing.T) {
	got := evaluateBreatheContext([]string{"echo {{agent-name}}"}, "yuki", "guion")
	if !strings.Contains(got, "yuki") {
		t.Errorf("expected output to contain agent name 'yuki', got: %q", got)
	}
}

func TestEvaluateBreatheContext_TeamNameExpansion(t *testing.T) {
	got := evaluateBreatheContext([]string{"echo {{team-name}}"}, "yuki", "guion")
	if !strings.Contains(got, "guion") {
		t.Errorf("expected output to contain team name 'guion', got: %q", got)
	}
}

func TestEvaluateBreatheContext_SuccessfulCommand(t *testing.T) {
	got := evaluateBreatheContext([]string{"echo hello"}, "yuki", "guion")
	if !strings.Contains(got, "hello") {
		t.Errorf("expected output to contain 'hello', got: %q", got)
	}
	// Should have the header format
	if !strings.Contains(got, "--- echo hello ---") {
		t.Errorf("expected header in output, got: %q", got)
	}
}

func TestEvaluateBreatheContext_FailedCommandNonFatal(t *testing.T) {
	// false exits with code 1; echo hello succeeds — both should run, only hello in output
	got := evaluateBreatheContext([]string{"false", "echo hello"}, "yuki", "guion")
	if !strings.Contains(got, "hello") {
		t.Errorf("expected 'hello' in output after failed command, got: %q", got)
	}
}

func TestEvaluateBreatheContext_MultiCommandSeparator(t *testing.T) {
	// Two successful commands: verify each gets its own header block separated correctly.
	got := evaluateBreatheContext([]string{"echo alpha", "echo beta"}, "yuki", "guion")
	if !strings.Contains(got, "--- echo alpha ---\nalpha") {
		t.Errorf("expected alpha block with header, got: %q", got)
	}
	if !strings.Contains(got, "--- echo beta ---\nbeta") {
		t.Errorf("expected beta block with header, got: %q", got)
	}
	// Verify the two sections are separated by a blank line
	if !strings.Contains(got, "alpha\n\n--- echo beta ---") {
		t.Errorf("expected blank line between sections, got: %q", got)
	}
}

func TestEvaluateBreatheContext_AllCommandsFail(t *testing.T) {
	got := evaluateBreatheContext([]string{"false", "exit 1"}, "yuki", "guion")
	if got != "" {
		t.Errorf("expected empty string when all commands fail, got: %q", got)
	}
}

func TestEvaluateBreatheContext_UnknownVarsPreserved(t *testing.T) {
	// Unknown template vars should be passed through literally
	got := evaluateBreatheContext([]string{"echo {{unknown}}"}, "yuki", "guion")
	if !strings.Contains(got, "{{unknown}}") {
		t.Errorf("expected unknown template var to be preserved literally, got: %q", got)
	}
}

func TestExpandBreatheVars(t *testing.T) {
	tests := []struct {
		name      string
		cmd       string
		agentName string
		teamName  string
		want      string
	}{
		{
			name:      "agent name replaced",
			cmd:       "diary {{agent-name}} read --last 1",
			agentName: "yuki",
			teamName:  "guion",
			want:      "diary yuki read --last 1",
		},
		{
			name:      "team name replaced",
			cmd:       "ttal team {{team-name}}",
			agentName: "yuki",
			teamName:  "guion",
			want:      "ttal team guion",
		},
		{
			name:      "both replaced",
			cmd:       "cmd {{agent-name}} {{team-name}}",
			agentName: "kestrel",
			teamName:  "default",
			want:      "cmd kestrel default",
		},
		{
			name:      "unknown vars preserved",
			cmd:       "cmd {{unknown}} {{agent-name}}",
			agentName: "yuki",
			teamName:  "guion",
			want:      "cmd {{unknown}} yuki",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := expandBreatheVars(tt.cmd, tt.agentName, tt.teamName)
			if got != tt.want {
				t.Errorf("expandBreatheVars() = %q, want %q", got, tt.want)
			}
		})
	}
}
