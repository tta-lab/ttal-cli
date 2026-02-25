package runtime

import "testing"

func TestFormatSkillInvocation(t *testing.T) {
	tests := []struct {
		rt    Runtime
		skill string
		want  string
	}{
		{ClaudeCode, "triage", "/triage"},
		{OpenCode, "triage", "/triage"},
		{Codex, "triage", "$triage"},
		{ClaudeCode, "review-pr", "/review-pr"},
		{Codex, "review-pr", "$review-pr"},
	}
	for _, tt := range tests {
		got := FormatSkillInvocation(tt.rt, tt.skill)
		if got != tt.want {
			t.Errorf("FormatSkillInvocation(%q, %q) = %q, want %q", tt.rt, tt.skill, got, tt.want)
		}
	}
}

func TestFormatSkillMessage(t *testing.T) {
	tests := []struct {
		rt    Runtime
		skill string
		msg   string
		want  string
	}{
		{ClaudeCode, "triage", "PR review posted.", "/triage PR review posted."},
		{OpenCode, "triage", "PR review posted.", "/triage PR review posted."},
		{Codex, "triage", "PR review posted.", "$triage PR review posted."},
	}
	for _, tt := range tests {
		got := FormatSkillMessage(tt.rt, tt.skill, tt.msg)
		if got != tt.want {
			t.Errorf("FormatSkillMessage(%q, %q, %q) = %q, want %q", tt.rt, tt.skill, tt.msg, got, tt.want)
		}
	}
}
