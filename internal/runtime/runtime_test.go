package runtime

import (
	"testing"
)

func TestParse(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    Runtime
		wantErr bool
	}{
		{"empty defaults to claude-code", "", ClaudeCode, false},
		{"claude-code literal", "claude-code", ClaudeCode, false},
		{"cc alias", "cc", ClaudeCode, false},
		{"opencode literal", "opencode", OpenCode, false},
		{"oc alias", "oc", OpenCode, false},
		{"codex literal", "codex", Codex, false},
		{"cx alias", "cx", Codex, false},
		{"unknown errors", "vim", "", true},
		{"partial match errors", "claude", "", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := Parse(tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("Parse(%q) error = %v, wantErr %v", tt.input, err, tt.wantErr)
				return
			}
			if got != tt.want {
				t.Errorf("Parse(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestValidate(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		wantErr bool
	}{
		{"claude-code valid", "claude-code", false},
		{"opencode valid", "opencode", false},
		{"codex valid", "codex", false},
		{"alias not valid for Validate", "cc", true},
		{"empty not valid for Validate", "", true},
		{"unknown not valid", "neovim", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := Validate(tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("Validate(%q) error = %v, wantErr %v", tt.input, err, tt.wantErr)
			}
		})
	}
}

func TestAll(t *testing.T) {
	all := All()
	if len(all) != 3 {
		t.Fatalf("All() returned %d runtimes, want 3", len(all))
	}
	if all[0] != ClaudeCode {
		t.Errorf("All()[0] = %q, want %q", all[0], ClaudeCode)
	}
	if all[1] != OpenCode {
		t.Errorf("All()[1] = %q, want %q", all[1], OpenCode)
	}
	if all[2] != Codex {
		t.Errorf("All()[2] = %q, want %q", all[2], Codex)
	}
}

func TestValues(t *testing.T) {
	vals := Values()
	if len(vals) != 3 {
		t.Fatalf("Values() returned %d strings, want 3", len(vals))
	}
	if vals[0] != "claude-code" {
		t.Errorf("Values()[0] = %q, want %q", vals[0], "claude-code")
	}
	if vals[1] != "opencode" {
		t.Errorf("Values()[1] = %q, want %q", vals[1], "opencode")
	}
	if vals[2] != "codex" {
		t.Errorf("Values()[2] = %q, want %q", vals[2], "codex")
	}
}
