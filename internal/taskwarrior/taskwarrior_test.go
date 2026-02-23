package taskwarrior

import (
	"errors"
	"testing"
)

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
		{
			"64-char boundary truncation",
			"this-is-a-slug-that-is-exactly-long-enough-to-hit-the-sixty-four-char-boundary-and-beyond",
			64,
			"this-is-a-slug-that-is-exactly-long-enough-to-hit-the-sixty",
		},
		{
			"64-char boundary exact fit",
			"abcdefghijklmnopqrstuvwxyz-abcdefghijklmnopqrstuvwxyz-abcdefghij",
			64,
			"abcdefghijklmnopqrstuvwxyz-abcdefghijklmnopqrstuvwxyz-abcdefghij",
		},
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
			"w-e9d4b7c1-add-ttal-doctor-command-scaffold",
		},
		{
			"no branch no description",
			"",
			"",
			"w-e9d4b7c1",
		},
		{
			"long branch name preserved",
			"feat/this-is-a-very-long-branch-name-that-should-be-truncated",
			"",
			"w-e9d4b7c1-this-is-a-very-long-branch-name-that-should-be-truncated",
		},
		{
			"long description preserved",
			"",
			"deploy secrets-ui to local k3s with cloudflare tunnel",
			"w-e9d4b7c1-deploy-secrets-ui-to-local-k3s-with-cloudflare-tunnel",
		},
		{
			"slug truncated at 64 chars",
			"feat/implement-very-long-feature-name-that-exceeds-sixty-four-characters-and-should-be-truncated",
			"",
			"w-e9d4b7c1-implement-very-long-feature-name-that-exceeds-sixty-four",
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
			// Session names should be reasonable length
			if len(got) > 80 {
				t.Errorf("SessionName() length %d exceeds 80", len(got))
			}
		})
	}
}

func TestValidateUUID(t *testing.T) {
	tests := []struct {
		name          string
		input         string
		wantErr       bool
		wantUserError bool // expect *UserError (formatted CLI guidance)
	}{
		{"full UUID", "e9d4b7c1-1234-5678-9abc-def012345678", false, false},
		{"8-char hex mixed", "e9d4b7c1", false, false},
		{"8-char hex all digits", "95502130", false, false},
		{"8-char hex sequential digits", "12345678", false, false},
		{"short numeric 2-char rejected", "42", true, true},
		{"short numeric 6-char rejected", "123456", true, true},
		{"short numeric 7-char rejected", "1234567", true, true},
		{"9-char numeric rejected", "123456789", true, true},
		{"hash prefix rejected", "#42", true, true},
		{"hash before valid hex rejected", "#e9d4b7c1", true, true},
		{"uppercase hex rejected", "E9D4B7C1", true, true},
		{"invalid string rejected", "not-a-uuid", true, true},
		{"empty rejected", "", true, false},
		{"spaces only rejected", "   ", true, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateUUID(tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateUUID(%q) error = %v, wantErr %v", tt.input, err, tt.wantErr)
			}
			if tt.wantUserError {
				var ue *UserError
				if !errors.As(err, &ue) {
					t.Errorf("ValidateUUID(%q) expected *UserError, got %T", tt.input, err)
				}
			}
		})
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
