package taskwarrior

import (
	"encoding/json"
	"fmt"
	"os/exec"
	"strings"
)

func ExportTask(uuid string) (*Task, error) {
	out, err := runTask(uuid, "export")
	if err != nil {
		return nil, fmt.Errorf("task not found in taskwarrior\n  UUID: %s\n  %w", uuid, err)
	}
	return parseFirstTask(out)
}

func ExportTaskBySessionID(sessionID, status string) (*Task, error) {
	var args []string
	if status != "" {
		args = append(args, fmt.Sprintf("status:%s", status))
	}
	args = append(args, fmt.Sprintf("uuid:%s", sessionID), "export")

	out, err := runTask(args...)
	if err != nil {
		return nil, fmt.Errorf("no task found with uuid prefix %s: %w", sessionID, err)
	}
	return parseFirstTask(out)
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

func ListTasksWithPR() ([]Task, error) {
	out, err := runTask("+ACTIVE", "pr_id.isnt:", "export")
	if err != nil {
		return nil, fmt.Errorf("failed to list tasks with PR: %w", err)
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

func GetActiveWorkerTasks() ([]Task, error) {
	out, err := runTask("status:pending", "+ACTIVE", "branch.any:", "export")
	if err != nil {
		return nil, fmt.Errorf("failed to query active worker tasks: %w", err)
	}

	out = strings.TrimSpace(out)
	if out == "" || out == "[]" {
		return nil, nil
	}

	var tasks []Task
	if err := json.Unmarshal([]byte(out), &tasks); err != nil {
		return nil, fmt.Errorf("failed to parse task JSON: %w", err)
	}
	return tasks, nil
}

func GetProjects() ([]string, error) {
	cmd := exec.Command("task", "_projects")
	out, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("failed to get projects: %w", err)
	}
	return parseSimpleListOutput(string(out)), nil
}

func GetTags() ([]string, error) {
	cmd := exec.Command("task", "_tags")
	out, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("failed to get tags: %w", err)
	}
	return parseSimpleListOutput(string(out)), nil
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
