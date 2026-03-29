package today

import (
	"encoding/json"
	"fmt"
	"log"
	"sort"
	"strings"
	"time"

	"charm.land/lipgloss/v2"
	"charm.land/lipgloss/v2/table"
	"github.com/tta-lab/ttal-cli/internal/format"
	"github.com/tta-lab/ttal-cli/internal/taskwarrior"
)

// List shows pending tasks scheduled for today or earlier, sorted by urgency.
func List() error {
	out, err := taskwarrior.Command("status:pending", "export").Output()
	if err != nil {
		return fmt.Errorf("failed to export tasks: %w", err)
	}

	var tasks []taskwarrior.Task
	if err := json.Unmarshal(out, &tasks); err != nil {
		return fmt.Errorf("failed to parse tasks: %w", err)
	}

	today := time.Now().Truncate(24 * time.Hour)
	var filtered []taskwarrior.Task
	for _, t := range tasks {
		if t.Scheduled == "" {
			continue
		}
		schedDate, err := taskwarrior.ParseTaskDate(t.Scheduled)
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
		return filtered[i].Urgency > filtered[j].Urgency
	})

	dimColor, headerStyle, cellStyle, dimStyle := format.TableStyles()

	rows := make([][]string, 0, len(filtered))
	for _, t := range filtered {
		due := ""
		if t.Due != "" {
			if parsed, err := taskwarrior.ParseTaskDate(t.Due); err == nil {
				due = parsed.Format("2006-01-02")
			}
		}
		rows = append(rows, []string{
			t.SessionID(),
			fmt.Sprintf("%.1f", t.Urgency),
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
			// Dim the metadata columns (ID [hex], Urg, Due)
			switch col {
			case 0, 1, 4:
				return dimStyle
			default:
				return cellStyle
			}
		}).
		Headers("ID", "Urg", "Project", "Tags", "Due", "Description").
		Rows(rows...)

	fmt.Println(tbl)
	fmt.Printf("\n%d %s\n", len(filtered), format.Plural(len(filtered), "task", "tasks"))
	return nil
}

// Completed shows tasks completed today.
func Completed() error {
	out, err := taskwarrior.Command("status:completed", "end:today", "export").Output()
	if err != nil {
		return fmt.Errorf("failed to export tasks: %w", err)
	}

	var tasks []taskwarrior.Task
	if err := json.Unmarshal(out, &tasks); err != nil {
		return fmt.Errorf("failed to parse tasks: %w", err)
	}

	if len(tasks) == 0 {
		fmt.Println("No tasks completed today.")
		return nil
	}

	sort.Slice(tasks, func(i, j int) bool {
		return tasks[i].End > tasks[j].End
	})

	dimColor, headerStyle, cellStyle, dimStyle := format.TableStyles()

	rows := make([][]string, 0, len(tasks))
	for _, t := range tasks {
		rows = append(rows, []string{
			t.SessionID(),
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
		Headers("ID", "Project", "Tags", "Description").
		Rows(rows...)

	fmt.Println(tbl)
	fmt.Printf("\n%d %s\n", len(tasks), format.Plural(len(tasks), "task", "tasks"))
	return nil
}

// Add sets scheduled:today on the given task IDs.
func Add(ids []string) error {
	if err := validateIDs(ids); err != nil {
		return err
	}
	for _, id := range ids {
		out, err := taskwarrior.Command(id, "modify", "scheduled:today").CombinedOutput()
		if err != nil {
			fmt.Printf("Error adding task %s: %s\n", id, strings.TrimSpace(string(out)))
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
		out, err := taskwarrior.Command(id, "modify", "scheduled:").CombinedOutput()
		if err != nil {
			fmt.Printf("Error removing task %s: %s\n", id, strings.TrimSpace(string(out)))
			continue
		}
		fmt.Printf("Task %s removed from today\n", id)
	}
	return nil
}

// CompletedCounts returns a map of date → completed task count for the past year.
// Date keys are truncated to midnight UTC for consistent lookups.
func CompletedCounts() (map[time.Time]int, error) {
	out, err := taskwarrior.Command("status:completed", "end.after:today-1y", "export").Output()
	if err != nil {
		return nil, fmt.Errorf("query completed tasks: %w", err)
	}

	var tasks []taskwarrior.Task
	if err := json.Unmarshal(out, &tasks); err != nil {
		return nil, fmt.Errorf("parse completed tasks: %w", err)
	}

	counts := make(map[time.Time]int)
	for _, t := range tasks {
		if t.End == "" {
			continue
		}
		end, err := taskwarrior.ParseTaskDate(t.End)
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

func validateIDs(ids []string) error {
	for _, id := range ids {
		if err := taskwarrior.ValidateUUID(id); err != nil {
			return err
		}
	}
	return nil
}
