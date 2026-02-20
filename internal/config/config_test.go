package config

import "testing"

func TestAgentSessionName(t *testing.T) {
	tests := []struct {
		agent string
		want  string
	}{
		{"kestrel", "session-kestrel"},
		{"yuki", "session-yuki"},
		{"athena", "session-athena"},
	}
	for _, tt := range tests {
		if got := AgentSessionName(tt.agent); got != tt.want {
			t.Errorf("AgentSessionName(%q) = %q, want %q", tt.agent, got, tt.want)
		}
	}
}
