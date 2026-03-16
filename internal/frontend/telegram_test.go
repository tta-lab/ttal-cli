package frontend

import (
	"testing"
)

func TestBuildSkillCommand(t *testing.T) {
	tests := []struct {
		name        string
		cmdName     string
		messageText string
		want        string
	}{
		{
			name:        "command with no args",
			cmdName:     "triage",
			messageText: "/triage",
			want:        "Use triage skill",
		},
		{
			name:        "command with args",
			cmdName:     "triage",
			messageText: "/triage fix this",
			want:        "Use triage skill. fix this",
		},
		{
			name:        "command with multiple args",
			cmdName:     "brainstorm",
			messageText: "/brainstorm new feature ideas",
			want:        "Use brainstorm skill. new feature ideas",
		},
		{
			name:        "hyphenated skill name with args",
			cmdName:     "write-plan",
			messageText: "/write_plan implement auth",
			want:        "Use write-plan skill. implement auth",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := buildSkillCommand(tt.cmdName, tt.messageText)
			if got != tt.want {
				t.Errorf("buildSkillCommand(%q, %q) = %q, want %q", tt.cmdName, tt.messageText, got, tt.want)
			}
		})
	}
}
