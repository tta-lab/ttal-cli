package team

import (
	"strings"
	"testing"
)

func TestBuildClaudeCommand(t *testing.T) {
	tests := []struct {
		name        string
		tab         AgentTab
		wantContain []string
		wantAbsent  []string
	}{
		{
			name:        "basic command",
			tab:         AgentTab{Name: "kestrel", Path: "/home/user/project"},
			wantContain: []string{"claude --dangerously-skip-permissions"},
			wantAbsent:  []string{"--model", "--continue"},
		},
		{
			name:        "with model",
			tab:         AgentTab{Name: "yuki", Path: "/tmp/work", Model: "sonnet"},
			wantContain: []string{"--model sonnet"},
			wantAbsent:  []string{"--continue"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := buildClaudeCommand(tt.tab)

			for _, want := range tt.wantContain {
				if !strings.Contains(got, want) {
					t.Errorf("command missing %q, got: %s", want, got)
				}
			}
			for _, absent := range tt.wantAbsent {
				if strings.Contains(got, absent) {
					t.Errorf("command should not contain %q, got: %s", absent, got)
				}
			}
		})
	}
}
