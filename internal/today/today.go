package today

import (
	"encoding/json"
	"fmt"
	"os/exec"
	"regexp"
	"sort"
	"strings"
	"time"
)

var validTaskID = regexp.MustCompile(`^\d+$`)

// task represents a taskwarrior task from JSON export.
type task struct {
	ID          int      `json:"id"`
	Description string   `json:"description"`
	Project     string   `json:"project"`
	Tags        []string `json:"tags"`
	Urgency     float64  `json:"urgency"`
	Due         string   `json:"due"`
	Scheduled   string   `json:"scheduled"`
	End         string   `json:"end"`
	Status      string   `json:"status"`
}

// List shows pending tasks scheduled for today or earlier, sorted by urgency.
func List() error {
	out, err := exec.Command("task", "status:pending", "export").Output()
	if err != nil {
		return fmt.Errorf("failed to export tasks: %w", err)
	}

	var tasks []task
	if err := json.Unmarshal(out, &tasks); err != nil {
		return fmt.Errorf("failed to parse tasks: %w", err)
	}

	today := time.Now().Truncate(24 * time.Hour)
	var filtered []task
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
		return filtered[i].Urgency > filtered[j].Urgency
	})

	printTaskTable(filtered)
	return nil
}

// Completed shows tasks completed today.
func Completed() error {
	out, err := exec.Command("task", "status:completed", "end:today", "export").Output()
	if err != nil {
		return fmt.Errorf("failed to export tasks: %w", err)
	}

	var tasks []task
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

	fmt.Printf("ID  Project      Tags                    Description\n")
	fmt.Printf("--- ------------ ----------------------- --------------------------\n")
	for _, t := range tasks {
		desc := truncate(t.Description, 40)
		tags := strings.Join(t.Tags, " ")
		fmt.Printf("%-3d %-12s %-23s %s\n", t.ID, truncate(t.Project, 12), truncate(tags, 23), desc)
	}
	fmt.Printf("\n%d %s\n", len(tasks), plural(len(tasks), "task", "tasks"))
	return nil
}

// Add sets scheduled:today on the given task IDs.
func Add(ids []string) error {
	if err := validateIDs(ids); err != nil {
		return err
	}
	for _, id := range ids {
		out, err := exec.Command("task", id, "modify", "scheduled:today").CombinedOutput()
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
		out, err := exec.Command("task", id, "modify", "scheduled:").CombinedOutput()
		if err != nil {
			fmt.Printf("Error removing task %s: %s\n", id, strings.TrimSpace(string(out)))
			continue
		}
		fmt.Printf("Task %s removed from today\n", id)
	}
	return nil
}

func printTaskTable(tasks []task) {
	fmt.Printf("ID  Urg   Project      Tags                    Due        Description\n")
	fmt.Printf("--- ----- ------------ ----------------------- ---------- --------------------------\n")
	for _, t := range tasks {
		due := ""
		if t.Due != "" {
			if parsed, err := parseTaskDate(t.Due); err == nil {
				due = parsed.Format("2006-01-02")
			}
		}
		tags := strings.Join(t.Tags, " ")
		desc := truncate(t.Description, 40)
		fmt.Printf("%-3d %-5.1f %-12s %-23s %-10s %s\n",
			t.ID, t.Urgency, truncate(t.Project, 12), truncate(tags, 23), due, desc)
	}
	fmt.Printf("\n%d %s\n", len(tasks), plural(len(tasks), "task", "tasks"))
}

// parseTaskDate parses taskwarrior date formats (ISO 8601 with T and Z).
func parseTaskDate(s string) (time.Time, error) {
	// Taskwarrior exports dates as "20260224T120000Z"
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
		if !validTaskID.MatchString(id) {
			return fmt.Errorf("invalid task ID: %q (must be numeric)", id)
		}
	}
	return nil
}

func truncate(s string, max int) string {
	runes := []rune(s)
	if len(runes) <= max {
		return s
	}
	return string(runes[:max-3]) + "..."
}

func plural(n int, singular, p string) string {
	if n == 1 {
		return singular
	}
	return p
}
