package status

import (
	"fmt"
	"time"
)

// FormatAge returns a humanized duration since t — the time-since-last-activity
// rendering used by /status replies. Bands (floor each): <60s → Xs, <60m → Xm,
// <24h → Xh, ≥24h → Xd. Future timestamps clamp to "0s". Zero time returns a
// huge Xd value (caller is responsible for distinguishing the missing-file case
// before calling).
func FormatAge(t time.Time) string {
	return formatDuration(time.Since(t))
}

// formatDuration is the deterministic core for FormatAge — testable without
// depending on time.Now().
func formatDuration(d time.Duration) string {
	if d < 0 {
		d = 0
	}
	switch {
	case d < time.Minute:
		return fmt.Sprintf("%ds", int(d.Seconds()))
	case d < time.Hour:
		return fmt.Sprintf("%dm", int(d.Minutes()))
	case d < 24*time.Hour:
		return fmt.Sprintf("%dh", int(d.Hours()))
	default:
		return fmt.Sprintf("%dd", int(d.Hours())/24)
	}
}
