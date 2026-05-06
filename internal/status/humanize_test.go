package status

import (
	"testing"
	"time"
)

func TestFormatDuration(t *testing.T) {
	tests := []struct {
		name string
		d    time.Duration
		want string
	}{
		{"zero", 0, "0s"},
		{"sub-minute mid", 30 * time.Second, "30s"},
		{"sub-minute upper bound", 59 * time.Second, "59s"},
		{"exactly 1 minute", 60 * time.Second, "1m"},
		{"sub-hour mid", 5 * time.Minute, "5m"},
		{"sub-hour upper bound", 59 * time.Minute, "59m"},
		{"exactly 1 hour", 60 * time.Minute, "1h"},
		{"sub-day mid floors", 1*time.Hour + 30*time.Minute, "1h"},
		{"sub-day upper bound", 23 * time.Hour, "23h"},
		{"exactly 1 day", 24 * time.Hour, "1d"},
		{"multi-day", 5 * 24 * time.Hour, "5d"},
		{"negative clamps", -10 * time.Second, "0s"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := formatDuration(tt.d)
			if got != tt.want {
				t.Errorf("formatDuration(%v) = %q, want %q", tt.d, got, tt.want)
			}
		})
	}
}

func TestFormatAge_RecentTimestamp(t *testing.T) {
	got := FormatAge(time.Now().Add(-5 * time.Second))
	if got != "5s" && got != "4s" && got != "6s" {
		t.Errorf("FormatAge(5s ago) = %q, want %q (or adjacent second)", got, "5s")
	}
}

func TestFormatAge_Zero(t *testing.T) {
	// Documented behavior: FormatAge has no defensive guard for zero time.
	// We do NOT assert the exact value (it is the X-day value of
	// time.Since(time.Time{}) which depends on the wall clock at test
	// execution). We only verify that the result has the Xd suffix shape.
	got := FormatAge(time.Time{})
	if len(got) < 2 || got[len(got)-1] != 'd' {
		t.Errorf("FormatAge(zero time) = %q, want a Xd value", got)
	}
}
