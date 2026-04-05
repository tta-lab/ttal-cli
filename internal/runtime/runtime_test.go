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
		{"codex literal", "codex", Codex, false},
		{"cx alias", "cx", Codex, false},
		{"lenos literal", "lenos", Lenos, false},
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
		{"codex valid", "codex", false},
		{"lenos valid", "lenos", false},
		{"alias not valid for Validate", "cc", true},
		{"alias not valid for Validate", "cx", true},
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
	want := []Runtime{ClaudeCode, Codex, Lenos}
	for i, w := range want {
		if all[i] != w {
			t.Errorf("All()[%d] = %q, want %q", i, all[i], w)
		}
	}
}

func TestValues(t *testing.T) {
	vals := Values()
	if len(vals) != 3 {
		t.Fatalf("Values() returned %d strings, want 3", len(vals))
	}
	want := []string{"claude-code", "codex", "lenos"}
	for i, w := range want {
		if vals[i] != w {
			t.Errorf("Values()[%d] = %q, want %q", i, vals[i], w)
		}
	}
}

func TestIsWorkerRuntime(t *testing.T) {
	if got := ClaudeCode.IsWorkerRuntime(); got != true {
		t.Errorf("ClaudeCode.IsWorkerRuntime() = %v, want true", got)
	}
	if got := Codex.IsWorkerRuntime(); got != true {
		t.Errorf("Codex.IsWorkerRuntime() = %v, want true", got)
	}
	if got := Lenos.IsWorkerRuntime(); got != true {
		t.Errorf("Lenos.IsWorkerRuntime() = %v, want true", got)
	}
}
