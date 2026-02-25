package enrichment

import "testing"

func TestGenerateBranch(t *testing.T) {
	tests := []struct {
		description string
		want        string
	}{
		{"Fix authentication timeout in login flow", "fix-authentication-timeout-login"},
		{"Add voice dictation push-to-talk", "add-voice-dictation-push"},
		{"Implement ttal doctor --fix auto-create", "implement-ttal-doctor-fix"},
		{"Update the README with new instructions", "update-readme-new-instructions"},
		{"Replace claude CLI enrichment with API", "replace-claude-cli-enrichment"},
		{"Fix bug", "fix-bug"},
		{"Add tests", "add-tests"},
		{"Do it for the win", "win"},
		{"", ""},
		// Single-char words filtered out
		{"a", "a"},
		{"I go", "go"},
		{"Fix the --verbose flag in CLI", "fix-verbose-flag-cli"},
		{"Add v2 API endpoint", "add-v2-api-endpoint"},
		{"Refactor the entire authentication system to use JWT tokens with refresh", "refactor-entire-authentication-system"},
	}

	for _, tt := range tests {
		t.Run(tt.description, func(t *testing.T) {
			got := GenerateBranch(tt.description)
			if got != tt.want {
				t.Errorf("GenerateBranch(%q) = %q, want %q", tt.description, got, tt.want)
			}
		})
	}
}

func TestGenerateBranch_MaxLength(t *testing.T) {
	long := "internationalization implementation containerization orchestration"
	got := GenerateBranch(long)
	if len(got) > maxBranchLen {
		t.Errorf("branch too long: %d chars (%q), max %d", len(got), got, maxBranchLen)
	}
}

func TestExtractWords(t *testing.T) {
	tests := []struct {
		input string
		want  []string
	}{
		{"hello world", []string{"hello", "world"}},
		{"fix-auth-timeout", []string{"fix", "auth", "timeout"}},
		{"add --verbose flag", []string{"add", "verbose", "flag"}},
		{"v2 api", []string{"v2", "api"}},
		{"", nil},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := extractWords(tt.input)
			if len(got) != len(tt.want) {
				t.Errorf("extractWords(%q) = %v, want %v", tt.input, got, tt.want)
				return
			}
			for i := range got {
				if got[i] != tt.want[i] {
					t.Errorf("extractWords(%q)[%d] = %q, want %q", tt.input, i, got[i], tt.want[i])
				}
			}
		})
	}
}
