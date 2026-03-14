package gitprovider

import (
	"testing"
)

func TestIsWoodpeckerContext(t *testing.T) {
	tests := []struct {
		context  string
		expected bool
	}{
		{"ci/woodpecker/pr/lint", true},
		{"ci/woodpecker/push/build", true},
		{"ci/jenkins", false},
		{"lint", false},
		{"", false},
	}
	for _, tt := range tests {
		if got := IsWoodpeckerContext(tt.context); got != tt.expected {
			t.Errorf("IsWoodpeckerContext(%q) = %v, want %v", tt.context, got, tt.expected)
		}
	}
}
