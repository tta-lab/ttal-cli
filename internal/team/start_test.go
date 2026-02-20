package team

import (
	"strings"
	"testing"
)

func TestBuildAgentLayoutKDL(t *testing.T) {
	tests := []struct {
		name        string
		tab         AgentTab
		hasContinue bool
		wantContain []string
		wantAbsent  []string
	}{
		{
			name:        "basic layout",
			tab:         AgentTab{Name: "kestrel", Path: "/home/user/project"},
			hasContinue: false,
			wantContain: []string{
				`tab name="kestrel" focus=true`,
				`cwd "/home/user/project"`,
				`args "-C" "claude --dangerously-skip-permissions"`,
				`tab name="term"`,
			},
			wantAbsent: []string{"--model", "--continue"},
		},
		{
			name:        "with model",
			tab:         AgentTab{Name: "yuki", Path: "/tmp/work", Model: "sonnet"},
			hasContinue: false,
			wantContain: []string{
				`tab name="yuki" focus=true`,
				`cwd "/tmp/work"`,
				"--model sonnet",
			},
			wantAbsent: []string{"--continue"},
		},
		{
			name:        "with continue",
			tab:         AgentTab{Name: "athena", Path: "/opt/code"},
			hasContinue: true,
			wantContain: []string{
				`tab name="athena" focus=true`,
				"--continue",
			},
			wantAbsent: []string{"--model"},
		},
		{
			name:        "with model and continue",
			tab:         AgentTab{Name: "kestrel", Path: "/work", Model: "opus"},
			hasContinue: true,
			wantContain: []string{
				"--model opus",
				"--continue",
				"claude --dangerously-skip-permissions --model opus --continue",
			},
		},
		{
			name:        "term tab uses agent path",
			tab:         AgentTab{Name: "kestrel", Path: "/specific/path"},
			hasContinue: false,
			wantContain: []string{
				`cwd "/specific/path"`,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := buildAgentLayoutKDL(tt.tab, tt.hasContinue)

			for _, want := range tt.wantContain {
				if !strings.Contains(got, want) {
					t.Errorf("layout missing %q\n\ngot:\n%s", want, got)
				}
			}
			for _, absent := range tt.wantAbsent {
				if strings.Contains(got, absent) {
					t.Errorf("layout should not contain %q\n\ngot:\n%s", absent, got)
				}
			}
		})
	}
}
