package today

import (
	"fmt"
	"log"
	"sort"
	"strings"
	"time"

	"charm.land/lipgloss/v2"
	"charm.land/lipgloss/v2/table"
	"github.com/tta-lab/ttal-cli/internal/flicktask"
	"github.com/tta-lab/ttal-cli/internal/format"
)

// List shows pending tasks scheduled for today or earlier, sorted by scheduled date.
func List() error {
	tasks, err := flicktask.ExportAll(false)
	if err != nil {
		return fmt.Errorf("failed to export tasks: %w", err)
	}

	today := time.Now().Truncate(24 * time.Hour)
	var filtered []flicktask.Task
	for _, t := range tasks {
		if t.Scheduled == "" {
			continue
		}
		schedDate, err := parseTaskDate(t.Scheduled)
		if err != nil {
			continue
		}
		if !schedDate.After(today) {
			filtered = append(filtered, t)
		}
	}

	if len(filtered) == 0 {
		fmt.Println("No tasks scheduled for today.")
		return nil
	}

	sort.Slice(filtered, func(i, j int) bool {
		return filtered[i].Scheduled < filtered[j].Scheduled
	})

	dimColor, headerStyle, cellStyle, dimStyle := format.TableStyles()

	rows := make([][]string, 0, len(filtered))
	for _, t := range filtered {
		due := ""
		if t.Due != "" {
			if parsed, err := parseTaskDate(t.Due); err == nil {
				due = parsed.Format("2006-01-02")
			}
		}
		rows = append(rows, []string{
			shortUUID(t.UUID),
			t.Project,
			strings.Join(t.Tags, " "),
			due,
			t.Description,
		})
	}

	tbl := table.New().
		Border(lipgloss.RoundedBorder()).
		BorderStyle(lipgloss.NewStyle().Foreground(dimColor)).
		StyleFunc(func(row, col int) lipgloss.Style {
			if row == table.HeaderRow {
				return headerStyle
			}
			// Dim the metadata columns (UUID, Due)
			switch col {
			case 0, 3:
				return dimStyle
			default:
				return cellStyle
			}
		}).
		Headers("UUID", "Project", "Tags", "Due", "Description").
		Rows(rows...)

	fmt.Println(tbl)
	fmt.Printf("\n%d %s\n", len(filtered), format.Plural(len(filtered), "task", "tasks"))
	return nil
}

// Completed shows tasks completed today.
func Completed() error {
	tasks, err := flicktask.ExportAll(true)
	if err != nil {
		return fmt.Errorf("failed to export tasks: %w", err)
	}

	today := time.Now().Truncate(24 * time.Hour)
	var filtered []flicktask.Task
	for _, t := range tasks {
		if t.End == "" {
			continue
		}
		end, err := parseTaskDate(t.End)
		if err != nil {
			continue
		}
		if !end.Before(today) {
			filtered = append(filtered, t)
		}
	}

	if len(filtered) == 0 {
		fmt.Println("No tasks completed today.")
		return nil
	}

	sort.Slice(filtered, func(i, j int) bool {
		return filtered[i].End > filtered[j].End
	})

	dimColor, headerStyle, cellStyle, dimStyle := format.TableStyles()

	rows := make([][]string, 0, len(filtered))
	for _, t := range filtered {
		rows = append(rows, []string{
			shortUUID(t.UUID),
			t.Project,
			strings.Join(t.Tags, " "),
			t.Description,
		})
	}

	tbl := table.New().
		Border(lipgloss.RoundedBorder()).
		BorderStyle(lipgloss.NewStyle().Foreground(dimColor)).
		StyleFunc(func(row, col int) lipgloss.Style {
			if row == table.HeaderRow {
				return headerStyle
			}
			if col == 0 {
				return dimStyle
			}
			return cellStyle
		}).
		Headers("UUID", "Project", "Tags", "Description").
		Rows(rows...)

	fmt.Println(tbl)
	fmt.Printf("\n%d %s\n", len(filtered), format.Plural(len(filtered), "task", "tasks"))
	return nil
}

// Add sets scheduled:today on the given task IDs.
func Add(ids []string) error {
	if err := validateIDs(ids); err != nil {
		return err
	}
	for _, id := range ids {
		if err := flicktask.EditScheduled(id, "today"); err != nil {
			fmt.Printf("Error adding task %s: %v\n", id, err)
			continue
		}
		fmt.Printf("Task %s added to today\n", id)
	}
	return nil
}

// Remove clears the scheduled date on the given task IDs.
func Remove(ids []string) error {
	if err := validateIDs(ids); err != nil {
		return err
	}
	for _, id := range ids {
		if err := flicktask.ClearScheduled(id); err != nil {
			fmt.Printf("Error removing task %s: %v\n", id, err)
			continue
		}
		fmt.Printf("Task %s removed from today\n", id)
	}
	return nil
}

// CompletedCounts returns a map of date → completed task count for the past year.
// Date keys are truncated to midnight UTC for consistent lookups.
func CompletedCounts() (map[time.Time]int, error) {
	tasks, err := flicktask.ExportAll(true)
	if err != nil {
		return nil, fmt.Errorf("query completed tasks: %w", err)
	}

	oneYearAgo := time.Now().UTC().AddDate(-1, 0, 0)
	counts := make(map[time.Time]int)
	for _, t := range tasks {
		if t.End == "" {
			continue
		}
		end, err := parseTaskDate(t.End)
		if err != nil {
			log.Printf("CompletedCounts: skipping task %s: cannot parse end date %q: %v", t.UUID, t.End, err)
			continue
		}
		if end.Before(oneYearAgo) {
			continue
		}
		day := time.Date(end.Year(), end.Month(), end.Day(), 0, 0, 0, 0, time.UTC)
		counts[day]++
	}

	return counts, nil
}

// parseTaskDate parses flicktask date formats (ISO 8601 with T and Z).
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
	return time.Time{}, fmt.Errorf("cannot parse date: %s", s)
}

func validateIDs(ids []string) error {
	for _, id := range ids {
		if err := flicktask.ValidateID(id); err != nil {
			return err
		}
	}
	return nil
}

func shortUUID(uuid string) string {
	if len(uuid) >= 8 {
		return uuid[:8]
	}
	return uuid
}
