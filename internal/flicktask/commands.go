package flicktask

import (
	"fmt"
	"strings"
	"time"
)

// AddOption configures optional fields for AddTask.
type AddOption func(*addOptions)

type addOptions struct {
	project   string
	tags      []string
	priority  string
	scheduled string
	udas      map[string]string
}

func WithProject(p string) AddOption  { return func(o *addOptions) { o.project = p } }
func WithTag(t string) AddOption      { return func(o *addOptions) { o.tags = append(o.tags, t) } }
func WithPriority(p string) AddOption { return func(o *addOptions) { o.priority = p } }
func WithScheduled(s string) AddOption {
	return func(o *addOptions) { o.scheduled = s }
}
func WithUDA(key, value string) AddOption {
	return func(o *addOptions) {
		if o.udas == nil {
			o.udas = make(map[string]string)
		}
		o.udas[key] = value
	}
}

// AddTask creates a new flicktask task and returns its ID.
func AddTask(description string, opts ...AddOption) (string, error) {
	o := &addOptions{}
	for _, opt := range opts {
		opt(o)
	}

	args := []string{"add", description}
	if o.project != "" {
		args = append(args, "--project", o.project)
	}
	for _, t := range o.tags {
		args = append(args, "--tag", t)
	}
	if o.priority != "" {
		args = append(args, "--priority", o.priority)
	}
	if o.scheduled != "" {
		args = append(args, "--scheduled", o.scheduled)
	}
	for k, v := range o.udas {
		args = append(args, "--set", k+"="+v)
	}

	out, err := runFlicktask(args...)
	if err != nil {
		return "", fmt.Errorf("failed to create task: %w", err)
	}

	id, err := parseCreatedID(out)
	if err != nil {
		return "", err
	}
	return id, nil
}

// parseCreatedID extracts the 8-char hex ID from flicktask add output.
func parseCreatedID(output string) (string, error) {
	// flicktask outputs something like "Created task abc12345"
	for _, line := range strings.Split(output, "\n") {
		fields := strings.Fields(line)
		for _, f := range fields {
			if hexIDPattern.MatchString(f) {
				return f, nil
			}
		}
	}
	return "", fmt.Errorf("could not find task ID in flicktask output: %q", output)
}

// AnnotateTask adds an annotation to a task.
func AnnotateTask(id, text string) error {
	_, err := runFlicktask("annotate", id, text)
	if err != nil {
		return fmt.Errorf("failed to annotate task %s: %w", id, err)
	}
	return nil
}

// StartTask marks a task as started (no-op in flicktask; uses edit --set start=now pattern).
func StartTask(id string) error {
	// flicktask doesn't have a dedicated start command; use edit with start UDA
	now := time.Now().UTC().Format("20060102T150405Z")
	_, err := runFlicktask("edit", id, "--set", "start="+now)
	if err != nil {
		return fmt.Errorf("failed to start task %s: %w", id, err)
	}
	return nil
}

// MarkDone marks a single task as done.
func MarkDone(id string) error {
	_, err := runFlicktask("done", id)
	if err != nil {
		return fmt.Errorf("failed to mark task %s as done: %w", id, err)
	}
	return nil
}

// MarkDoneRecursive marks a task and all its subtasks as done.
func MarkDoneRecursive(id string) error {
	_, err := runFlicktask("done", "--recursive", id)
	if err != nil {
		return fmt.Errorf("failed to mark task %s (recursive) as done: %w", id, err)
	}
	return nil
}

// MarkDeleted deletes a task (requires --yes to skip confirmation).
func MarkDeleted(id string) error {
	_, err := runFlicktask("delete", "--yes", id)
	if err != nil {
		return fmt.Errorf("failed to delete task %s: %w", id, err)
	}
	return nil
}

// UpdateWorkerMetadata sets the branch UDA on a task.
func UpdateWorkerMetadata(id, branch string) error {
	_, err := runFlicktask("edit", id, "--set", "branch="+branch)
	if err != nil {
		return fmt.Errorf("failed to assign worker metadata to task %s: %w", id, err)
	}
	return nil
}

// SetSpawner sets the spawner UDA on a task (format: team:agent).
func SetSpawner(id, spawner string) error {
	_, err := runFlicktask("edit", id, "--set", "spawner="+spawner)
	if err != nil {
		return fmt.Errorf("failed to set spawner on task %s: %w", id, err)
	}
	return nil
}

// SetPRID sets the pr_id UDA on a task.
func SetPRID(id, prID string) error {
	_, err := runFlicktask("edit", id, "--set", "pr_id="+prID)
	if err != nil {
		return fmt.Errorf("failed to set pr_id on task %s: %w", id, err)
	}
	return nil
}

// SetPRLGTM appends :lgtm to the task's pr_id UDA.
func SetPRLGTM(id string) error {
	task, err := ExportTask(id)
	if err != nil {
		return fmt.Errorf("failed to read task %s: %w", id, err)
	}
	if task.PRID == "" {
		return fmt.Errorf("task %s has no pr_id", id)
	}
	if strings.HasSuffix(task.PRID, ":lgtm") {
		return nil // already approved
	}
	return SetPRID(id, task.PRID+":lgtm")
}

// EditScheduled sets the scheduled date on a task.
func EditScheduled(id, date string) error {
	_, err := runFlicktask("edit", id, "--scheduled", date)
	if err != nil {
		return fmt.Errorf("failed to set scheduled on task %s: %w", id, err)
	}
	return nil
}

// ClearScheduled clears the scheduled date on a task.
func ClearScheduled(id string) error {
	_, err := runFlicktask("edit", id, "--scheduled", "")
	if err != nil {
		return fmt.Errorf("failed to clear scheduled on task %s: %w", id, err)
	}
	return nil
}
