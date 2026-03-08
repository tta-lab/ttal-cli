package cmd

import "testing"

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
