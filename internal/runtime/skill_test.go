package runtime

import "testing"

func TestFormatSkillInvocation(t *testing.T) {
	tests := []struct {
		rt    Runtime
		skill string
		want  string
	}{
		{ClaudeCode, "triage", "run ttal skill get triage"},
		{Codex, "triage", "$triage"},
		{ClaudeCode, "review-pr", "run ttal skill get review-pr"},
		{Codex, "review-pr", "$review-pr"},
	}
	for _, tt := range tests {
		got := FormatSkillInvocation(tt.rt, tt.skill)
		if got != tt.want {
			t.Errorf("FormatSkillInvocation(%q, %q) = %q, want %q", tt.rt, tt.skill, got, tt.want)
		}
	}
}
