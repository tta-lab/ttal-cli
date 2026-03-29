package taskwarrior

import (
	"errors"
	"testing"
	"time"
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
		name string
		desc string
		want string
	}{
		{
			"description used for slug",
			"feat(doctor): add ttal doctor command scaffold",
			"w-e9d4b7c1-add-ttal-doctor-command-scaffold",
		},
		{
			"empty description",
			"",
			"w-e9d4b7c1",
		},
		{
			"description is stable regardless of branch state",
			"fix: ttal open session fails with no worker session",
			"w-e9d4b7c1-ttal-open-session-fails-with-no-worker-session",
		},
		{
			"long description truncated at 64 chars",
			"deploy secrets-ui to local k3s with cloudflare tunnel and extra words that push past limit",
			"w-e9d4b7c1-deploy-secrets-ui-to-local-k3s-with-cloudflare-tunnel-and-extra",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			task := &Task{
				UUID:        testUUID,
				Description: tt.desc,
			}
			got := task.SessionName()
			if got != tt.want {
				t.Errorf("SessionName() = %q, want %q", got, tt.want)
			}
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

func TestShouldInlineNote(t *testing.T) {
	tests := []struct {
		name           string
		project        string
		inlineProjects []string
		want           bool
	}{
		{"plan project", "Task Plans", []string{"plan"}, true},
		{"plan lowercase", "plans", []string{"plan"}, true},
		{"research project", "Research Notes", []string{"plan"}, false},
		{"empty project", "", []string{"plan"}, false},
		{"unrelated project", "Backend API", []string{"plan"}, false},
		{"plan substring", "deployment-planning", []string{"plan"}, true},
		{"multi-keyword match fix", "ttal.fixes", []string{"plan", "fix"}, true},
		{"multi-keyword match plan", "ttal.plans", []string{"plan", "fix"}, true},
		{"multi-keyword no match", "ttal.research", []string{"plan", "fix"}, false},
		{"empty filter list", "Task Plans", nil, false},
		{"case insensitive keyword", "Task Plans", []string{"Plan"}, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			note := &FlicknoteNote{Project: tt.project}
			got := ShouldInlineNote(note, tt.inlineProjects)
			if got != tt.want {
				t.Errorf("ShouldInlineNote(project=%q, filter=%v) = %v, want %v", tt.project, tt.inlineProjects, got, tt.want)
			}
		})
	}
}

func TestFormatFlicknoteContent(t *testing.T) {
	tests := []struct {
		name string
		note *FlicknoteNote
		want string
	}{
		{
			"title only",
			&FlicknoteNote{Title: "My Plan"},
			"Title: My Plan\n",
		},
		{
			"title and summary",
			&FlicknoteNote{Title: "My Plan", Summary: "A brief summary"},
			"Title: My Plan\nSummary: A brief summary\n",
		},
		{
			"full note",
			&FlicknoteNote{Title: "My Plan", Summary: "Brief", Content: "Full content here"},
			"Title: My Plan\nSummary: Brief\n\nFull content here",
		},
		{
			"title and content no summary",
			&FlicknoteNote{Title: "My Plan", Content: "Content only"},
			"Title: My Plan\n\nContent only",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := formatFlicknoteContent(tt.note)
			if got != tt.want {
				t.Errorf("formatFlicknoteContent() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestHexIDPattern(t *testing.T) {
	tests := []struct {
		name   string
		input  string
		wantID string
		wantOK bool
	}{
		{"bare hex", "e8fd0fe0", "e8fd0fe0", true},
		{"plan prefix", "Plan: e8fd0fe0", "e8fd0fe0", true},
		{"research prefix", "Research: abcd1234", "abcd1234", true},
		{"design prefix", "Design: 12345678abcdef", "12345678abcdef", true},
		{"no space after colon", "Plan:e8fd0fe0", "e8fd0fe0", true},
		{"multiple spaces", "Plan:  e8fd0fe0", "e8fd0fe0", true},
		{"flicknote prefix", "Plan: flicknote b7b61e89", "b7b61e89", true},
		{"multi word prefix", "Design: flicknote draft a1b2c3d4e5", "a1b2c3d4e5", true},
		{"path no match", "Plan: ~/docs/plan.md", "", false},
		{"too short hex", "Plan: abcd", "", false},
		{"uppercase hex no match", "Plan: E8FD0FE0", "", false},
		{"plain text no match", "This is a regular annotation", "", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := HexIDPattern.FindStringSubmatch(tt.input)
			if tt.wantOK {
				if len(m) < 2 {
					t.Fatalf("HexIDPattern did not match %q", tt.input)
				}
				if m[1] != tt.wantID {
					t.Errorf("HexIDPattern captured %q, want %q", m[1], tt.wantID)
				}
			} else {
				if len(m) > 0 {
					t.Errorf("HexIDPattern unexpectedly matched %q, captured %q", tt.input, m[1])
				}
			}
		})
	}
}

func TestParseCreatedUUID(t *testing.T) {
	tests := []struct {
		name    string
		output  string
		wantID  string
		wantErr bool
	}{
		{
			name:   "standard output",
			output: "Created task e9d4b7c1-1234-5678-9abc-def012345678.\n",
			wantID: "e9d4b7c1-1234-5678-9abc-def012345678",
		},
		{
			name:    "empty output",
			output:  "",
			wantErr: true,
		},
		{
			name:   "uuid in verbose output",
			output: "Created task e9d4b7c1-1234-5678-9abc-def012345678 (waiting for hook).\n",
			wantID: "e9d4b7c1-1234-5678-9abc-def012345678",
		},
		{
			name:    "no uuid in output",
			output:  "Some other output without a uuid.\n",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parseCreatedUUID(tt.output)
			if tt.wantErr {
				if err == nil {
					t.Error("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != tt.wantID {
				t.Errorf("got %q, want %q", got, tt.wantID)
			}
		})
	}
}

func TestExtractHexID(t *testing.T) {
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
			got := ExtractHexID(tt.input)
			if got != tt.want {
				t.Errorf("ExtractHexID(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestIsLGTMTag(t *testing.T) {
	tests := []struct {
		tag  string
		want bool
	}{
		{"plan_lgtm", true},
		{"implement_lgtm", true},
		{"_lgtm", true},
		{"lgtm", false}, // no underscore prefix — not a stage lgtm tag
		{"feature", false},
		{"", false},
		{"plan_lgtm_extra", false}, // does not end with _lgtm
	}
	for _, tt := range tests {
		if got := IsLGTMTag(tt.tag); got != tt.want {
			t.Errorf("IsLGTMTag(%q) = %v, want %v", tt.tag, got, tt.want)
		}
	}
}

func TestHasAnyLGTMTag(t *testing.T) {
	tests := []struct {
		name string
		tags []string
		want bool
	}{
		{"empty slice", []string{}, false},
		{"bare lgtm — not a stage tag", []string{"lgtm"}, false},
		{"plan_lgtm present", []string{"feature", "plan_lgtm"}, true},
		{"implement_lgtm present", []string{"plan_lgtm", "implement_lgtm"}, true},
		{"only unrelated tags", []string{"feature", "urgent"}, false},
		{"mixed — one lgtm tag", []string{"feature", "plan", "plan_lgtm"}, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := HasAnyLGTMTag(tt.tags); got != tt.want {
				t.Errorf("HasAnyLGTMTag(%v) = %v, want %v", tt.tags, got, tt.want)
			}
		})
	}
}

func TestParseTaskDate(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		wantErr bool
	}{
		{"compact format", "20260224T120000Z", false},
		{"RFC3339", "2026-02-24T12:00:00Z", false},
		{"ISO with Z", "2026-02-24T12:00:00Z", false},
		{"date only", "2026-02-24", false},
		{"invalid", "not-a-date", true},
		{"empty", "", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := ParseTaskDate(tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("ParseTaskDate(%q) error = %v, wantErr %v", tt.input, err, tt.wantErr)
			}
		})
	}
}

func TestIsToday(t *testing.T) {
	today := time.Now().UTC()
	yesterday := today.Add(-24 * time.Hour)
	tomorrow := today.Add(24 * time.Hour)

	tests := []struct {
		name      string
		scheduled string
		want      bool
	}{
		{"empty", "", false},
		{"today", today.Format("20060102T150405Z"), true},
		{"yesterday", yesterday.Format("20060102T150405Z"), true},
		{"tomorrow", tomorrow.Format("20060102T150405Z"), false},
		{"invalid date", "bad-date", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			task := Task{Scheduled: tt.scheduled}
			if got := task.IsToday(); got != tt.want {
				t.Errorf("IsToday() with Scheduled=%q = %v, want %v", tt.scheduled, got, tt.want)
			}
		})
	}
}

func TestFormatAge(t *testing.T) {
	tests := []struct {
		name string
		d    time.Duration
		want string
	}{
		{"minutes", 45 * time.Minute, "45m"},
		{"one hour", time.Hour, "1h"},
		{"hours rounds up", 1*time.Hour + 55*time.Minute, "2h"},
		{"hours", 5 * time.Hour, "5h"},
		{"one day", 24 * time.Hour, "1d"},
		{"days", 10 * 24 * time.Hour, "10d"},
		{"29 days", 29 * 24 * time.Hour, "29d"},
		{"30 days becomes months", 30 * 24 * time.Hour, "1mo"},
		{"months", 90 * 24 * time.Hour, "3mo"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := formatAge(tt.d); got != tt.want {
				t.Errorf("formatAge(%v) = %q, want %q", tt.d, got, tt.want)
			}
		})
	}
}
