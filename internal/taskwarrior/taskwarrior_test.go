package taskwarrior

import "testing"

const testUUID = "e9d4b7c1-1234-5678-9abc-def012345678"

func TestSlugify(t *testing.T) {
	tests := []struct {
		name   string
		input  string
		maxLen int
		want   string
	}{
		{"branch with feat prefix", "feat/fix-auth-flow", 24, "fix-auth-flow"},
		{"branch with worker prefix", "worker/molt", 24, "molt"},
		{"branch with fix prefix", "fix/timeout-bug", 24, "timeout-bug"},
		{"conventional commit desc", "feat(doctor): add ttal doctor command scaffold", 24, "add-ttal-doctor-command"},
		{"plain description", "add user authentication", 24, "add-user-authentication"},
		{"special chars cleaned", "feat/fix_auth--flow!!!", 24, "fix-auth-flow"},
		{
			"truncation at word boundary",
			"this-is-a-very-long-slug-that-exceeds-the-max-length", 24,
			"this-is-a-very-long",
		},
		{"empty input", "", 24, ""},
		{"only special chars", "!!!", 24, ""},
		{"refactor prefix", "refactor/clean-up-tests", 24, "clean-up-tests"},
		{"docs prefix", "docs/update-readme", 24, "update-readme"},
		{"chore prefix", "chore/bump-deps", 24, "bump-deps"},
		{"colon prefix", "fix: resolve panic on nil", 24, "resolve-panic-on-nil"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := slugify(tt.input, tt.maxLen)
			if got != tt.want {
				t.Errorf("slugify(%q, %d) = %q, want %q", tt.input, tt.maxLen, got, tt.want)
			}
		})
	}
}

func TestSessionName(t *testing.T) {
	tests := []struct {
		name   string
		branch string
		desc   string
		want   string
	}{
		{
			"branch preferred over description",
			"feat/fix-auth-flow",
			"some description",
			"w-e9d4b7c1-fix-auth-flow",
		},
		{
			"worker branch prefix stripped",
			"worker/molt",
			"molt description",
			"w-e9d4b7c1-molt",
		},
		{
			"description fallback",
			"",
			"feat(doctor): add ttal doctor command scaffold",
			"w-e9d4b7c1-add-ttal-doctor-command",
		},
		{
			"no branch no description",
			"",
			"",
			"w-e9d4b7c1",
		},
		{
			"max length respected",
			"feat/this-is-a-very-long-branch-name-that-should-be-truncated",
			"",
			"w-e9d4b7c1-this-is-a-very-long",
		},
		{
			"socket path safe for deploy slug",
			"",
			"deploy secrets-ui to local k3s with cloudflare tunnel",
			"w-e9d4b7c1-deploy-secrets-ui-to",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			task := &Task{
				UUID:        testUUID,
				Branch:      tt.branch,
				Description: tt.desc,
			}
			got := task.SessionName()
			if got != tt.want {
				t.Errorf("SessionName() = %q, want %q", got, tt.want)
			}
			if len(got) > DefaultMaxSessionLen {
				t.Errorf("SessionName() length %d exceeds max %d", len(got), DefaultMaxSessionLen)
			}
		})
	}
}

func TestSessionNameWithLimit(t *testing.T) {
	task := &Task{
		UUID:        testUUID,
		Description: "deploy secrets-ui to local k3s with cloudflare tunnel",
	}

	// With limit 37 (typical macOS budget), should fit
	got := task.SessionNameWithLimit(37)
	if len(got) > 37 {
		t.Errorf("SessionNameWithLimit(37) = %q (len %d), exceeds 37", got, len(got))
	}

	// With limit 50, should get a longer name
	long := task.SessionNameWithLimit(50)
	if len(long) <= len(got) && len(task.Description) > 37 {
		t.Errorf("SessionNameWithLimit(50) = %q should be longer than (37) = %q", long, got)
	}
}

func TestExtractSessionID(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{"new format", "w-e9d4b7c1-fix-auth", "e9d4b7c1"},
		{"new format no slug", "w-e9d4b7c1", "e9d4b7c1"},
		{"old format bare uuid", "e9d4b7c1", "e9d4b7c1"},
		{"empty string", "", ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ExtractSessionID(tt.input)
			if got != tt.want {
				t.Errorf("ExtractSessionID(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}
