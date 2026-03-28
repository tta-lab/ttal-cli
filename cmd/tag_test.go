package cmd

import "testing"

func TestSemverRe(t *testing.T) {
	tests := []struct {
		tag   string
		valid bool
	}{
		// Valid
		{"v1.0.0", true},
		{"v0.1.0", true},
		{"v10.20.30", true},
		{"v1.0.0-rc.1", true},
		{"v1.0.0-alpha.1", true},
		{"v1.1.1-guion.1", true},
		{"v1.0.0+build.123", true},
		{"v1.0.0-rc.1+build.456", true},
		{"v1.0.0-beta", true},

		// Invalid
		{"1.0.0", false},          // missing v prefix
		{"v1.0", false},           // missing patch
		{"v1.0.a", false},         // non-numeric patch
		{"v1.0.0-", false},        // trailing hyphen
		{"v1.0.0+", false},        // trailing plus
		{"v1.0.0-rc-1", false},    // hyphen within pre-release segment
		{"v1.0.0-alpha-1", false}, // hyphen within pre-release segment
		{"latest", false},         // not semver
		{"", false},               // empty
		{"v", false},              // just prefix
	}

	for _, tt := range tests {
		t.Run(tt.tag, func(t *testing.T) {
			got := semverRe.MatchString(tt.tag)
			if got != tt.valid {
				t.Errorf("semverRe.MatchString(%q) = %v, want %v", tt.tag, got, tt.valid)
			}
		})
	}
}
