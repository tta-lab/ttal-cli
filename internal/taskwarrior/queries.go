package taskwarrior

import (
	"encoding/json"
	"fmt"
	"strings"
)

func ExportTask(uuid string) (*Task, error) {
	out, err := runTask(uuid, "export")
	if err != nil {
		return nil, fmt.Errorf("task not found in taskwarrior\n  UUID: %s\n  %w", uuid, err)
	}
	return parseFirstTask(out)
}

func ExportTaskByHexID(hexID, status string) (*Task, error) {
	var args []string
	if status != "" {
		args = append(args, fmt.Sprintf("status:%s", status))
	}
	args = append(args, fmt.Sprintf("uuid:%s", hexID), "export")

	out, err := runTask(args...)
	if err != nil {
		return nil, fmt.Errorf("no task found with uuid prefix %s: %w", hexID, err)
	}
	task, err := parseFirstTask(out)
	if err != nil {
		return nil, fmt.Errorf("no task found with uuid prefix %s: %w", hexID, err)
	}
	return task, nil
}

func FindTasks(keywords []string, status string) ([]Task, error) {
	parts := make([]string, len(keywords))
	for i, kw := range keywords {
		quoted := `"` + strings.ReplaceAll(kw, `"`, `\"`) + `"`
		parts[i] = "description.contains:" + quoted
	}
	filter := "(" + strings.Join(parts, " or ") + ")"

	args := []string{filter}
	if status != "" {
		args = append(args, fmt.Sprintf("status:%s", status))
	}
	args = append(args, "export")

	out, err := runTask(args...)
	if err != nil {
		return nil, fmt.Errorf("failed to search tasks: %w", err)
	}

	var tasks []Task
	if err := json.Unmarshal([]byte(strings.TrimSpace(out)), &tasks); err != nil {
		return nil, fmt.Errorf("failed to parse task JSON (output: %q): %w", out, err)
	}
	return tasks, nil
}

// exportTasks runs a task export query and unmarshals the JSON result.
// Returns nil, nil when there are no matching tasks.
func exportTasks(args ...string) ([]Task, error) {
	out, err := runTask(args...)
	if err != nil {
		return nil, err
	}
	trimmed := strings.TrimSpace(out)
	if trimmed == "" || trimmed == "[]" {
		return nil, nil
	}
	var tasks []Task
	if err := json.Unmarshal([]byte(trimmed), &tasks); err != nil {
		return nil, fmt.Errorf("failed to parse task JSON: %w", err)
	}
	return tasks, nil
}

// ExportTasksByFilter runs a task export with the given filter args.
func ExportTasksByFilter(args ...string) ([]Task, error) {
	return exportTasks(append(args, "export")...)
}

func ListTasksWithPR() ([]Task, error) {
	tasks, err := exportTasks("+ACTIVE", "pr_id.isnt:", "export")
	if err != nil {
		return nil, fmt.Errorf("failed to list tasks with PR: %w", err)
	}
	return tasks, nil
}

func GetActiveWorkerTasks() ([]Task, error) {
	tasks, err := exportTasks("status:pending", "+ACTIVE", "spawner.any:", "export")
	if err != nil {
		return nil, fmt.Errorf("failed to query active worker tasks: %w", err)
	}
	return tasks, nil
}

func GetProjects() ([]string, error) {
	out, err := runTask("_projects")
	if err != nil {
		return nil, fmt.Errorf("failed to get projects: %w", err)
	}
	return parseSimpleListOutput(out), nil
}

func GetTags() ([]string, error) {
	out, err := runTask("_tags")
	if err != nil {
		return nil, fmt.Errorf("failed to get tags: %w", err)
	}
	return parseSimpleListOutput(out), nil
}

// GetDueReminders returns pending tasks tagged +reminder with scheduled <= now.
// Used by the daemon poller to fire notifications.
func GetDueReminders() ([]Task, error) {
	tasks, err := exportTasks("+reminder", "scheduled.before:now", "status:pending", "export")
	if err != nil {
		return nil, fmt.Errorf("failed to query due reminders: %w", err)
	}
	return tasks, nil
}

// GetPendingReminders returns all pending tasks tagged +reminder (for `ttal remind list`).
func GetPendingReminders() ([]Task, error) {
	tasks, err := exportTasks("+reminder", "status:pending", "export")
	if err != nil {
		return nil, fmt.Errorf("failed to query pending reminders: %w", err)
	}
	return tasks, nil
}

func parseSimpleListOutput(out string) []string {
	var result []string
	lines := strings.Split(out, "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line != "" {
			result = append(result, line)
		}
	}
	return result
}
