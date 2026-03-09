package cmd

import "testing"

func TestIsLGTMBody(t *testing.T) {
	tests := []struct {
		body string
		want bool
	}{
		{"LGTM", true},
		{"lgtm", true},
		{"Lgtm!", true},
		{"Looks good to me", true}, // "LOOKS GOOD" is a substring
		{"LOOKS GOOD", true},
		{"looks good", true},
		{"NOT LGTM", true}, // contains "LGTM" — by design, substring match
		{"approved, ready to merge", false},
		{"", false},
	}
	for _, tt := range tests {
		got := isLGTMBody(tt.body)
		if got != tt.want {
			t.Errorf("isLGTMBody(%q) = %v, want %v", tt.body, got, tt.want)
		}
	}
}
