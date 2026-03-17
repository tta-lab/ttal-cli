package flicktask

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

// ExportTask exports a single task by ID.
func ExportTask(id string) (*Task, error) {
	out, err := runFlicktask("export", id)
	if err != nil {
		return nil, fmt.Errorf("task not found in flicktask\n  ID: %s\n  %w", id, err)
	}
	return parseFirstTask(out)
}

// ExportTaskBySessionID finds a task by its 8-char session ID (UUID prefix).
// The status parameter is accepted for API compatibility but flicktask export
// filters by pending by default; use --completed for completed tasks.
func ExportTaskBySessionID(sessionID, status string) (*Task, error) {
	var args []string
	if status == "completed" {
		args = append(args, "export", "--completed", sessionID)
	} else {
		args = append(args, "export", sessionID)
	}

	out, err := runFlicktask(args...)
	if err != nil {
		// Try pending if status wasn't explicitly completed
		if status != "completed" {
			return nil, fmt.Errorf("no task found with ID prefix %s: %w", sessionID, err)
		}
		return nil, fmt.Errorf("no task found with ID prefix %s: %w", sessionID, err)
	}
	return parseFirstTask(out)
}

// FindTasks searches for tasks by keywords (OR match).
func FindTasks(keywords []string, completed bool) ([]Task, error) {
	args := []string{"find"}
	args = append(args, keywords...)
	if completed {
		args = append(args, "--completed")
	}

	out, err := runFlicktask(args...)
	if err != nil {
		return nil, fmt.Errorf("failed to search tasks: %w", err)
	}

	return parseTasks(out)
}

// exportAll runs flicktask export and returns all tasks.
func exportAll() ([]Task, error) {
	out, err := runFlicktask("export")
	if err != nil {
		return nil, err
	}
	return parseTasks(out)
}

// ListTasksWithPR returns active tasks that have a pr_id set.
func ListTasksWithPR() ([]Task, error) {
	tasks, err := exportAll()
	if err != nil {
		return nil, fmt.Errorf("failed to list tasks with PR: %w", err)
	}

	var result []Task
	for _, t := range tasks {
		if t.PRID != "" && (t.Status == "pending" || t.Status == "active") {
			result = append(result, t)
		}
	}
	return result, nil
}

// GetActiveWorkerTasks returns pending/active tasks that have a branch set.
func GetActiveWorkerTasks() ([]Task, error) {
	tasks, err := exportAll()
	if err != nil {
		return nil, fmt.Errorf("failed to query active worker tasks: %w", err)
	}

	var result []Task
	for _, t := range tasks {
		if t.Branch != "" && (t.Status == "pending" || t.Status == "active") {
			result = append(result, t)
		}
	}
	return result, nil
}

// GetDueReminders returns pending tasks tagged +reminder with scheduled <= now.
func GetDueReminders() ([]Task, error) {
	tasks, err := exportAll()
	if err != nil {
		return nil, fmt.Errorf("failed to query due reminders: %w", err)
	}

	now := time.Now().UTC()
	var result []Task
	for _, t := range tasks {
		if !t.HasTag("reminder") || t.Scheduled == "" {
			continue
		}
		scheduled, err := parseTaskTime(t.Scheduled)
		if err != nil {
			continue
		}
		if !scheduled.After(now) {
			result = append(result, t)
		}
	}
	return result, nil
}

// GetPendingReminders returns all pending tasks tagged +reminder.
func GetPendingReminders() ([]Task, error) {
	tasks, err := exportAll()
	if err != nil {
		return nil, fmt.Errorf("failed to query pending reminders: %w", err)
	}

	var result []Task
	for _, t := range tasks {
		if t.HasTag("reminder") {
			result = append(result, t)
		}
	}
	return result, nil
}

// GetProjects returns unique project names from all tasks.
func GetProjects() ([]string, error) {
	tasks, err := exportAll()
	if err != nil {
		return nil, fmt.Errorf("failed to get projects: %w", err)
	}

	seen := make(map[string]bool)
	var result []string
	for _, t := range tasks {
		if t.Project != "" && !seen[t.Project] {
			seen[t.Project] = true
			result = append(result, t.Project)
		}
	}
	return result, nil
}

// GetTags returns unique tag names from all tasks.
func GetTags() ([]string, error) {
	tasks, err := exportAll()
	if err != nil {
		return nil, fmt.Errorf("failed to get tags: %w", err)
	}

	seen := make(map[string]bool)
	var result []string
	for _, t := range tasks {
		for _, tag := range t.Tags {
			if !seen[tag] {
				seen[tag] = true
				result = append(result, tag)
			}
		}
	}
	return result, nil
}

func parseFirstTask(output string) (*Task, error) {
	output = strings.TrimSpace(output)
	if output == "" || output == "[]" {
		return nil, fmt.Errorf("no task found")
	}

	// flicktask export <id> may return a single object or array
	if strings.HasPrefix(output, "{") {
		var task Task
		if err := json.Unmarshal([]byte(output), &task); err != nil {
			return nil, fmt.Errorf("failed to parse task JSON: %w", err)
		}
		return &task, nil
	}

	var tasks []Task
	if err := json.Unmarshal([]byte(output), &tasks); err != nil {
		return nil, fmt.Errorf("failed to parse task JSON: %w", err)
	}
	if len(tasks) == 0 {
		return nil, fmt.Errorf("no task found")
	}
	return &tasks[0], nil
}

func parseTasks(output string) ([]Task, error) {
	trimmed := strings.TrimSpace(output)
	if trimmed == "" || trimmed == "[]" {
		return nil, nil
	}
	var tasks []Task
	if err := json.Unmarshal([]byte(trimmed), &tasks); err != nil {
		return nil, fmt.Errorf("failed to parse task JSON: %w", err)
	}
	return tasks, nil
}

// parseTaskTime parses a taskwarrior-style timestamp (20060102T150405Z).
func parseTaskTime(s string) (time.Time, error) {
	return time.Parse("20060102T150405Z", s)
}
