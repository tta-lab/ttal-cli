package cmdexec

import "testing"

func TestClassifyShellCmd(t *testing.T) {
	tests := []struct {
		name string
		cmd  string
		want string
	}{
		// ttal commands
		{
			name: "ttal send",
			cmd:  "ttal send --to foo",
			want: "ttal:send",
		},
		{
			name: "ttal go",
			cmd:  "ttal go abc123",
			want: "ttal:route",
		},

		// flicknote write commands
		{
			name: "flicknote add",
			cmd:  "flicknote add \"hello\"",
			want: "flicknote:write",
		},
		{
			name: "flicknote modify",
			cmd:  "flicknote modify abc123 --section Hb",
			want: "flicknote:write",
		},
		{
			name: "flicknote append",
			cmd:  "flicknote append abc123",
			want: "flicknote:write",
		},
		{
			name: "flicknote insert",
			cmd:  "flicknote insert abc123 --before xy",
			want: "flicknote:write",
		},
		{
			name: "flicknote delete",
			cmd:  "flicknote delete abc123",
			want: "flicknote:write",
		},
		{
			name: "flicknote rename",
			cmd:  "flicknote rename abc123 --section xy \"New Name\"",
			want: "flicknote:write",
		},

		// flicknote read commands
		{
			name: "flicknote detail",
			cmd:  "flicknote detail abc123",
			want: "flicknote:read",
		},
		{
			name: "flicknote content",
			cmd:  "flicknote content abc123",
			want: "flicknote:read",
		},
		{
			name: "flicknote list exact",
			cmd:  "flicknote list",
			want: "flicknote:read",
		},
		{
			name: "flicknote list with args",
			cmd:  "flicknote list --project plans",
			want: "flicknote:read",
		},
		{
			name: "flicknote find",
			cmd:  "flicknote find \"some query\"",
			want: "flicknote:read",
		},
		{
			name: "flicknote count",
			cmd:  "flicknote count",
			want: "flicknote:read",
		},

		// Pipe: flicknote after pipe (requires exact "| " pattern)
		{
			name: "pipe flicknote add",
			cmd:  "echo hi | flicknote add",
			want: "flicknote:write",
		},
		{
			name: "pipe flicknote find",
			cmd:  "echo hi | flicknote find foo",
			want: "flicknote:read",
		},

		// Unknown commands → Bash
		{
			name: "ls",
			cmd:  "ls -la",
			want: "Bash",
		},
		{
			name: "empty",
			cmd:  "",
			want: "Bash",
		},

		// Edge case: pipe without trailing space (no '| ' match → falls to Bash)
		{
			name: "pipe without space",
			cmd:  "grep foo|wc -l",
			want: "Bash",
		},

		// Edge case: pipe with extra spaces → TrimSpace strips leading spaces after |,
		// leaving "flicknote add" which matches "flicknote add " prefix
		{
			name: "pipe with extra spaces",
			cmd:  "echo x   |   flicknote add",
			want: "flicknote:write",
		},

		// Whitespace normalization
		{
			name: "leading whitespace",
			cmd:  "  ttal send --to bar",
			want: "ttal:send",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ClassifyShellCmd(tt.cmd)
			if got != tt.want {
				t.Errorf("ClassifyShellCmd(%q) = %q, want %q", tt.cmd, got, tt.want)
			}
		})
	}
}
