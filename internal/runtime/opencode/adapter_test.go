package opencode

import "testing"

func TestOcToolToToolName(t *testing.T) {
	tests := []struct {
		ocTool   string
		wantName string
	}{
		{"read", "Read"},
		{"glob", "Glob"},
		{"grep", "Grep"},
		{"edit", "Edit"},
		{"write", "Write"},
		{"bash", "Bash"},
		{"webSearch", "WebSearch"},
		{"webFetch", "WebFetch"},
		{"agent", "Agent"},
		{"someCustomTool", "someCustomTool"},
	}

	for _, tt := range tests {
		t.Run(tt.ocTool, func(t *testing.T) {
			got := ocToolToToolName(tt.ocTool)
			if got != tt.wantName {
				t.Errorf("ocToolToToolName(%q) = %q, want %q", tt.ocTool, got, tt.wantName)
			}
		})
	}
}
