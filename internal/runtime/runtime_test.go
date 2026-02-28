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
		{"openclaw literal", "openclaw", OpenClaw, false},
		{"oclw alias", "oclw", OpenClaw, false},
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
		{"openclaw valid", "openclaw", false},
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
	if len(all) != 4 {
		t.Fatalf("All() returned %d runtimes, want 4", len(all))
	}
	want := []Runtime{ClaudeCode, OpenCode, Codex, OpenClaw}
	for i, w := range want {
		if all[i] != w {
			t.Errorf("All()[%d] = %q, want %q", i, all[i], w)
		}
	}
}

func TestValues(t *testing.T) {
	vals := Values()
	if len(vals) != 4 {
		t.Fatalf("Values() returned %d strings, want 4", len(vals))
	}
	want := []string{"claude-code", "opencode", "codex", "openclaw"}
	for i, w := range want {
		if vals[i] != w {
			t.Errorf("Values()[%d] = %q, want %q", i, vals[i], w)
		}
	}
}

func TestNeedsPort(t *testing.T) {
	tests := []struct {
		rt   Runtime
		want bool
	}{
		{ClaudeCode, false},
		{OpenCode, true},
		{Codex, true},
		{OpenClaw, false},
	}
	for _, tt := range tests {
		if got := tt.rt.NeedsPort(); got != tt.want {
			t.Errorf("%s.NeedsPort() = %v, want %v", tt.rt, got, tt.want)
		}
	}
}

func TestIsWorkerRuntime(t *testing.T) {
	tests := []struct {
		rt   Runtime
		want bool
	}{
		{ClaudeCode, true},
		{OpenCode, true},
		{Codex, true},
		{OpenClaw, false},
	}
	for _, tt := range tests {
		if got := tt.rt.IsWorkerRuntime(); got != tt.want {
			t.Errorf("%s.IsWorkerRuntime() = %v, want %v", tt.rt, got, tt.want)
		}
	}
}
