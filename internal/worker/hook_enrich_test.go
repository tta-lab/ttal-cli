package worker

import "testing"

func TestParseBranchFromOutput(t *testing.T) {
	tests := []struct {
		name   string
		input  string
		expect string
	}{
		{
			name:   "simple branch name",
			input:  "fix-auth-timeout\n",
			expect: "fix-auth-timeout",
		},
		{
			name:   "with leading/trailing whitespace",
			input:  "  add-voice-config  \n",
			expect: "add-voice-config",
		},
		{
			name:   "skips comment lines",
			input:  "# Here is the branch name:\nfix-login-bug\n",
			expect: "fix-login-bug",
		},
		{
			name:   "skips empty lines",
			input:  "\n\nresolve-project-path\n",
			expect: "resolve-project-path",
		},
		{
			name:   "strips backticks",
			input:  "`fix-auth-timeout`\n",
			expect: "fix-auth-timeout",
		},
		{
			name:   "converts spaces to hyphens",
			input:  "fix auth timeout\n",
			expect: "fix-auth-timeout",
		},
		{
			name:   "lowercases output",
			input:  "Fix-Auth-Timeout\n",
			expect: "fix-auth-timeout",
		},
		{
			name:   "strips special characters",
			input:  "fix/auth_timeout!\n",
			expect: "fixauthtimeout",
		},
		{
			name:   "trims leading/trailing hyphens",
			input:  "-fix-auth-\n",
			expect: "fix-auth",
		},
		{
			name:   "empty input returns empty",
			input:  "",
			expect: "",
		},
		{
			name:   "only whitespace returns empty",
			input:  "  \n  \n",
			expect: "",
		},
		{
			name:   "only hyphens returns empty",
			input:  "---\n",
			expect: "",
		},
		{
			name:   "only comments returns empty",
			input:  "# comment\n# another\n",
			expect: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := parseBranchFromOutput(tt.input)
			if got != tt.expect {
				t.Errorf("parseBranchFromOutput(%q) = %q, want %q", tt.input, got, tt.expect)
			}
		})
	}
}
