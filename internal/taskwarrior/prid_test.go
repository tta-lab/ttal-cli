package taskwarrior

import "testing"

func TestParsePRID(t *testing.T) {
	tests := []struct {
		input string
		index int64
		lgtm  bool
		err   bool
	}{
		{"123", 123, false, false},
		{"123:lgtm", 123, true, false},
		{"", 0, false, true},
		{"abc", 0, false, true},
		{"123:other", 0, false, true},
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
		if info.LGTM != tt.lgtm {
			t.Errorf("ParsePRID(%q).LGTM = %v, want %v", tt.input, info.LGTM, tt.lgtm)
		}
	}
}
