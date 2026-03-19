package cmd

import "testing"

func TestShouldBreathe(t *testing.T) {
	tests := []struct {
		name      string
		role      string
		noBreathe bool
		want      bool
	}{
		{"designer is breathed", "designer", false, true},
		{"manager is exempt", "manager", false, false},
		{"no-breathe flag overrides", "designer", true, false},
		{"empty role is breathed", "", false, true},
		{"manager with no-breathe", "manager", true, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := shouldBreathe(tt.role, tt.noBreathe); got != tt.want {
				t.Errorf("shouldBreathe(%q, %v) = %v, want %v", tt.role, tt.noBreathe, got, tt.want)
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
