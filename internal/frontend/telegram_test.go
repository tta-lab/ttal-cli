package frontend

import (
	"testing"
)

func TestBuildFullCommand(t *testing.T) {
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
			want:        "/triage",
		},
		{
			name:        "command with args",
			cmdName:     "triage",
			messageText: "/triage fix this",
			want:        "/triage fix this",
		},
		{
			name:        "command with multiple args",
			cmdName:     "brainstorm",
			messageText: "/brainstorm new feature ideas",
			want:        "/brainstorm new feature ideas",
		},
		{
			name:        "hyphenated skill name with args",
			cmdName:     "write-plan",
			messageText: "/write_plan implement auth",
			want:        "/write-plan implement auth",
		},
		{
			name:        "command with @bot suffix in group chat",
			cmdName:     "write-plan",
			messageText: "/write_plan@somebot implement auth",
			want:        "/write-plan implement auth",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := buildFullCommand(tt.cmdName, tt.messageText)
			if got != tt.want {
				t.Errorf("buildFullCommand(%q, %q) = %q, want %q", tt.cmdName, tt.messageText, got, tt.want)
			}
		})
	}
}
