package team

import (
	"strings"
	"testing"

	"codeberg.org/clawteam/ttal-cli/internal/runtime"
)

func TestBuildAgentCommand(t *testing.T) {
	tests := []struct {
		name        string
		tab         AgentTab
		wantContain []string
		wantAbsent  []string
	}{
		{
			name:        "claude-code basic command",
			tab:         AgentTab{Name: "kestrel", Path: "/home/user/project", Runtime: runtime.ClaudeCode},
			wantContain: []string{"claude --dangerously-skip-permissions"},
			wantAbsent:  []string{"--model", "--continue", "opencode"},
		},
		{
			name:        "claude-code with model",
			tab:         AgentTab{Name: "yuki", Path: "/tmp/work", Model: "sonnet", Runtime: runtime.ClaudeCode},
			wantContain: []string{"--model sonnet"},
			wantAbsent:  []string{"--continue", "opencode"},
		},
		{
			name:        "empty runtime defaults to claude-code",
			tab:         AgentTab{Name: "kestrel", Path: "/home/user/project"},
			wantContain: []string{"claude --dangerously-skip-permissions"},
			wantAbsent:  []string{"opencode"},
		},
		{
			name:        "opencode runtime",
			tab:         AgentTab{Name: "sven", Path: "/tmp/work", Runtime: runtime.OpenCode},
			wantContain: []string{"opencode"},
			wantAbsent:  []string{"claude", "--dangerously-skip-permissions"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := buildAgentCommand(tt.tab)

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
