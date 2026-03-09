package tui

import (
	"fmt"
	"math"
	"time"

	"github.com/tta-lab/ttal-cli/internal/taskwarrior"
)

type Task struct {
	taskwarrior.Task
	Priority  string  `json:"priority,omitempty"`
	Urgency   float64 `json:"urgency"`
	Scheduled string  `json:"scheduled,omitempty"`
	Due       string  `json:"due,omitempty"`
	Entry     string  `json:"entry,omitempty"`
}

func (t *Task) ShortUUID() string {
	if len(t.UUID) >= 8 {
		return t.UUID[:8]
	}
	return t.UUID
}

func (t *Task) Age() string {
	if t.Entry == "" {
		return ""
	}
	parsed, err := time.Parse("20060102T150405Z", t.Entry)
	if err != nil {
		return "?"
	}
	return formatAge(time.Since(parsed))
}

func formatAge(d time.Duration) string {
	if d < time.Hour {
		return fmt.Sprintf("%dm", int(d.Minutes()))
	}
	if d < 24*time.Hour {
		return fmt.Sprintf("%dh", int(math.Round(d.Hours())))
	}
	if d < 30*24*time.Hour {
		return fmt.Sprintf("%dd", int(d.Hours()/24))
	}
	return fmt.Sprintf("%dmo", int(d.Hours()/24/30))
}

func (t *Task) IsActive() bool {
	return t.Start != ""
}

func (t *Task) IsToday() bool {
	if t.Scheduled == "" {
		return false
	}
	parsed, err := parseTaskDate(t.Scheduled)
	if err != nil {
		return false
	}
	today := time.Now().Truncate(24 * time.Hour)
	return !parsed.After(today)
}

func parseTaskDate(s string) (time.Time, error) {
	formats := []string{
		"20060102T150405Z",
		time.RFC3339,
		"2006-01-02T15:04:05Z",
		"2006-01-02",
	}
	for _, f := range formats {
		if t, err := time.Parse(f, s); err == nil {
			return t.Truncate(24 * time.Hour), nil
		}
	}
	return time.Time{}, fmt.Errorf("unparseable date: %s", s)
}
