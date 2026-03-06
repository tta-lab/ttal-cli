package runtime

import "testing"

func TestFormatSkillInvocation(t *testing.T) {
	tests := []struct {
		rt    Runtime
		skill string
		want  string
	}{
		{ClaudeCode, "triage", "Use triage skill"},
		{OpenCode, "triage", "Use triage skill"},
		{Codex, "triage", "$triage"},
		{ClaudeCode, "review-pr", "Use review-pr skill"},
		{Codex, "review-pr", "$review-pr"},
		// sp- skills should also work
		{ClaudeCode, "sp-executing-plans", "Use sp-executing-plans skill"},
		{Codex, "sp-executing-plans", "$sp-executing-plans"},
	}
	for _, tt := range tests {
		got := FormatSkillInvocation(tt.rt, tt.skill)
		if got != tt.want {
			t.Errorf("FormatSkillInvocation(%q, %q) = %q, want %q", tt.rt, tt.skill, got, tt.want)
		}
	}
}
