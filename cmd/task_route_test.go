package cmd

import "testing"

func TestShouldBreathe(t *testing.T) {
	// No status file exists for these agents, so missing status → breathe (safe fallback).
	// We test the guard clauses (manager, no-breathe) and the fallback behavior.
	tests := []struct {
		name      string
		agentName string
		role      string
		noBreathe bool
		threshold float64
		want      bool
	}{
		{"manager is exempt", "inke", "manager", false, 40, false},
		{"no-breathe flag overrides", "inke", "designer", true, 40, false},
		{"manager with no-breathe", "inke", "manager", true, 40, false},
		// Missing status file → safe fallback → breathe
		{"missing status defaults to breathe", "nonexistent-agent-xyz", "designer", false, 40, true},
		{"empty role with missing status defaults to breathe", "nonexistent-agent-xyz", "", false, 40, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := shouldBreathe(tt.agentName, tt.role, tt.noBreathe, tt.threshold); got != tt.want {
				t.Errorf("shouldBreathe(%q, %q, %v, %v) = %v, want %v",
					tt.agentName, tt.role, tt.noBreathe, tt.threshold, got, tt.want)
			}
		})
	}
}

func TestBuildRoutingRecord(t *testing.T) {
	tests := []struct {
		from, to, message, want string
	}{
		{
			from: "yuki", to: "inke", message: "focus on auth",
			want: "routed: yuki → inke [message: focus on auth]",
		},
		{
			from: "yuki", to: "inke", message: "",
			want: "routed: yuki → inke",
		},
		{
			from: "", to: "inke", message: "",
			want: "routed: unknown → inke",
		},
		{
			from: "", to: "athena", message: "deep dive on perf",
			want: "routed: unknown → athena [message: deep dive on perf]",
		},
	}
	for _, tt := range tests {
		got := buildRoutingRecord(tt.from, tt.to, tt.message)
		if got != tt.want {
			t.Errorf("buildRoutingRecord(%q, %q, %q) = %q, want %q",
				tt.from, tt.to, tt.message, got, tt.want)
		}
	}
}
