package taskwarrior

import "testing"

func TestParsePRID(t *testing.T) {
	tests := []struct {
		input string
		index int64
		err   bool
	}{
		{"123", 123, false},
		{"123:lgtm", 123, false}, // backward compat: strips :lgtm suffix
		{"", 0, true},
		{"abc", 0, true},
	}
	for _, tt := range tests {
		info, err := ParsePRID(tt.input)
		if tt.err {
			if err == nil {
				t.Errorf("ParsePRID(%q) expected error", tt.input)
			}
			continue
		}
		if err != nil {
			t.Errorf("ParsePRID(%q) unexpected error: %v", tt.input, err)
			continue
		}
		if info.Index != tt.index {
			t.Errorf("ParsePRID(%q).Index = %d, want %d", tt.input, info.Index, tt.index)
		}
	}
}
