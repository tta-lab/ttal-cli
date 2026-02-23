package worker

import (
	"testing"
)

func TestParseRuntime(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    Runtime
		wantErr bool
	}{
		{"empty defaults to claude-code", "", RuntimeClaudeCode, false},
		{"claude-code", "claude-code", RuntimeClaudeCode, false},
		{"cc alias", "cc", RuntimeClaudeCode, false},
		{"opencode", "opencode", RuntimeOpenCode, false},
		{"oc alias", "oc", RuntimeOpenCode, false},
		{"unknown errors", "vim", "", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ParseRuntime(tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("ParseRuntime(%q) error = %v, wantErr %v", tt.input, err, tt.wantErr)
				return
			}
			if got != tt.want {
				t.Errorf("ParseRuntime(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestValidateRuntime(t *testing.T) {
	// "claude" should be in PATH in CI/dev environments
	// We can't guarantee "opencode" is available, so just test the mapping logic
	tests := []struct {
		name    string
		runtime Runtime
		wantBin string
	}{
		{"claude-code maps to claude binary", RuntimeClaudeCode, "claude"},
		{"opencode maps to opencode binary", RuntimeOpenCode, "opencode"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateRuntime(tt.runtime)
			// We only check that it returns a meaningful error mentioning the binary
			// since we can't control what's in PATH during tests
			if err != nil && tt.wantBin != "" {
				if got := err.Error(); !contains(got, tt.wantBin) {
					t.Errorf("validateRuntime(%q) error = %q, should mention %q", tt.runtime, got, tt.wantBin)
				}
			}
		})
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && searchString(s, substr)
}

func searchString(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
