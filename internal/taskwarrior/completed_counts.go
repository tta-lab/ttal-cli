// Package taskwarrior provides shared taskwarrior helpers.
//
// Plane: shared
package taskwarrior

import (
	"encoding/json"
	"fmt"
	"log"
	"time"
)

// CompletedCounts returns a map of date → completed root-task count for the past year.
// Date keys are truncated to midnight UTC for consistent lookups.
func CompletedCounts() (map[time.Time]int, error) {
	out, err := Command("status:completed", "end.after:today-1y", "export").Output()
	if err != nil {
		return nil, fmt.Errorf("query completed tasks: %w", err)
	}

	var tasks []Task
	if err := json.Unmarshal(out, &tasks); err != nil {
		return nil, fmt.Errorf("parse completed tasks: %w", err)
	}

	counts := make(map[time.Time]int)
	for _, t := range tasks {
		// Skip subtasks — completed subtasks inflate the heatmap counts
		if t.ParentID != "" || t.End == "" {
			continue
		}
		end, err := ParseTaskDate(t.End)
		if err != nil {
			log.Printf("CompletedCounts: skipping task %s: cannot parse end date %q: %v", t.UUID, t.End, err)
			continue
		}
		// Truncate to midnight UTC — taskwarrior dates are UTC
		day := time.Date(end.Year(), end.Month(), end.Day(), 0, 0, 0, 0, time.UTC)
		counts[day]++
	}

	return counts, nil
}
