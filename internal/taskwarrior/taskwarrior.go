package taskwarrior

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

var uuidPattern = regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$`)

const cmdTimeout = 5 * time.Second

// UserError is an error with a user-facing message intended for CLI display.
// The message may contain newlines and formatting.
type UserError struct {
	Msg string
}

func (e *UserError) Error() string { return e.Msg }

func userError(format string, args ...any) error {
	return &UserError{Msg: fmt.Sprintf(format, args...)}
}

// Annotation represents a taskwarrior annotation.
type Annotation struct {
	Description string `json:"description"`
	Entry       string `json:"entry,omitempty"`
}

// Task represents a taskwarrior task with worker UDAs.
type Task struct {
	UUID        string       `json:"uuid"`
	Description string       `json:"description"`
	Project     string       `json:"project,omitempty"`
	Status      string       `json:"status"`
	Tags        []string     `json:"tags,omitempty"`
	Annotations []Annotation `json:"annotations,omitempty"`
	Start       string       `json:"start,omitempty"`
	Modified    string       `json:"modified,omitempty"`
	Branch      string       `json:"branch"`
	ProjectPath string       `json:"project_path"`
	PRID        string       `json:"pr_id,omitempty"`
}

// SessionID returns a deterministic session identifier derived from the task UUID.
// Uses the first 8 characters of the UUID (4 billion possible values).
func (t *Task) SessionID() string {
	if len(t.UUID) >= 8 {
		return t.UUID[:8]
	}
	return t.UUID
}

// HasTag returns true if the task has the given tag.
func (t *Task) HasTag(tag string) bool {
	for _, tt := range t.Tags {
		if tt == tag {
			return true
		}
	}
	return false
}

// fileRefPattern matches annotations like "Design: ~/path/to/file.md"
var fileRefPattern = regexp.MustCompile(`(?:Plan|Design|Doc|Reference|File):\s*([~\/][\w\/\-\.]+\.md)`)

// FormatPrompt formats the task for injection into a worker's Claude prompt.
// Includes description, annotations, and inlined referenced markdown docs.
func (t *Task) FormatPrompt() string {
	lines := make([]string, 0, 1+len(t.Annotations))

	lines = append(lines, t.Description)

	// Separate file-reference annotations from content annotations
	fileRefDescs := make(map[string]bool)
	type fileRef struct {
		label string
		path  string
	}
	var fileRefs []fileRef

	for _, ann := range t.Annotations {
		matches := fileRefPattern.FindAllStringSubmatch(ann.Description, -1)
		for _, m := range matches {
			fileRefDescs[ann.Description] = true
			fileRefs = append(fileRefs, fileRef{label: ann.Description, path: m[1]})
		}
	}

	// Content annotations (skip file refs, they're inlined below)
	for _, ann := range t.Annotations {
		if fileRefDescs[ann.Description] {
			continue
		}
		lines = append(lines, "")
		lines = append(lines, ann.Description)
	}

	result := strings.Join(lines, "\n") + "\n"

	// Inline referenced markdown files
	if len(fileRefs) > 0 {
		result += "\nReferenced Documentation:\n"
		sep := strings.Repeat("═", 80)
		subSep := strings.Repeat("─", 80)
		for _, ref := range fileRefs {
			result += sep + "\n"
			result += ref.label + "\n"
			result += subSep + "\n"
			result += readFileRef(ref.path) + "\n"
			result += sep + "\n"
		}
	}

	return result
}

// readFileRef reads a file path, expanding ~ to home directory.
func readFileRef(path string) string {
	if strings.HasPrefix(path, "~") {
		home, err := os.UserHomeDir()
		if err != nil {
			return fmt.Sprintf("[Error expanding home directory: %v]", err)
		}
		path = filepath.Join(home, path[1:])
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return fmt.Sprintf("[File not found: %s]", path)
	}
	return string(data)
}

// ValidateUUID checks that s is a valid taskwarrior UUID.
// Returns a user-friendly error for numeric IDs, # prefixes, or invalid formats.
func ValidateUUID(s string) error {
	s = strings.TrimSpace(s)

	if s == "" {
		return fmt.Errorf("task UUID is required")
	}

	if isNumeric(s) {
		return userError("numeric task IDs are no longer supported\n\n"+
			"  You provided: %s\n\n"+
			"  Numeric IDs are unstable (they change when tasks complete).\n"+
			"  Use the permanent UUID instead:\n\n"+
			"  # Get UUID for task #%s:\n"+
			"  task %s export | jq -r '.[0].uuid'\n\n"+
			"  # Then use the UUID:\n"+
			"  ttal worker spawn --task <uuid> ...", s, s, s)
	}

	if strings.HasPrefix(s, "#") {
		remaining := s[1:]
		if isNumeric(remaining) {
			return userError("# prefix format is no longer supported\n\n"+
				"  You provided: %s\n\n"+
				"  Use the bare UUID instead:\n\n"+
				"  # Get UUID for task #%s:\n"+
				"  task %s export | jq -r '.[0].uuid'", s, remaining, remaining)
		}
		return userError("# prefix format is no longer supported\n\n"+
			"  You provided: %s\n\n"+
			"  Remove the # prefix:\n"+
			"  ttal worker spawn --task %s ...", s, remaining)
	}

	if !uuidPattern.MatchString(s) {
		return userError("only UUIDs are supported for task spawning\n\n"+
			"  You provided: %s\n\n"+
			"  ttal worker spawn requires a taskwarrior UUID.\n"+
			"  This ensures all workers are tracked in taskwarrior.\n\n"+
			"  To spawn a worker:\n"+
			"  1. Create task: task add \"%s\" project:... +tag priority:H\n"+
			"  2. Get UUID: task export | jq -r '.[-1].uuid'\n"+
			"  3. Spawn: ttal worker spawn --task <uuid> ...", s, s)
	}

	return nil
}

// VerifyRequiredUDAs checks that branch and project_path UDAs
// are configured in taskwarrior.
func VerifyRequiredUDAs() error {
	out, err := runTask("show")
	if err != nil {
		return &UserError{Msg: fmt.Sprintf("could not verify UDA configuration: %v\n\n"+
			"  This prevents creating orphaned sessions that aren't tracked.", err)}
	}

	required := []string{"branch", "project_path"}
	var missing []string
	for _, uda := range required {
		if !strings.Contains(out, fmt.Sprintf("uda.%s.", uda)) {
			missing = append(missing, uda)
		}
	}

	if len(missing) > 0 {
		msg := fmt.Sprintf("required UDAs not configured in taskwarrior\n\n"+
			"  Missing UDAs: %s\n\n"+
			"  Add these to ~/.taskrc:", strings.Join(missing, ", "))
		for _, uda := range missing {
			label := strings.ReplaceAll(uda, "_", " ")
			msg += fmt.Sprintf("\n    uda.%s.type=string\n    uda.%s.label=%s", uda, uda, capitalizeWords(label))
		}
		return fmt.Errorf("%s", msg)
	}

	return nil
}

// ExportTask loads a task by UUID from taskwarrior.
func ExportTask(uuid string) (*Task, error) {
	out, err := runTask(uuid, "export")
	if err != nil {
		return nil, fmt.Errorf("task not found in taskwarrior\n  UUID: %s\n  %w", uuid, err)
	}
	return parseFirstTask(out)
}

// ExportTaskBySessionID finds a task by UUID prefix (first 8 chars).
// If status is non-empty, filters by that status.
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

// UpdateWorkerMetadata sets branch and project_path UDAs on a task.
func UpdateWorkerMetadata(uuid, branch, projectPath string) error {
	_, err := runTask(uuid, "modify",
		fmt.Sprintf("branch:%s", branch),
		fmt.Sprintf("project_path:%s", projectPath),
	)
	if err != nil {
		return fmt.Errorf("failed to assign worker metadata to task %s: %w", uuid, err)
	}
	return nil
}

// SetPRID sets the pr_id UDA on a task.
func SetPRID(uuid, prID string) error {
	_, err := runTask(uuid, "modify", fmt.Sprintf("pr_id:%s", prID))
	if err != nil {
		return fmt.Errorf("failed to set pr_id on task %s: %w", uuid, err)
	}
	return nil
}

// AnnotateTask adds an annotation to a task.
func AnnotateTask(uuid, message string) error {
	_, err := runTask(uuid, "annotate", message)
	if err != nil {
		return fmt.Errorf("failed to annotate task %s: %w", uuid, err)
	}
	return nil
}

// MarkDone marks a task as completed.
func MarkDone(uuid string) error {
	_, err := runTask(uuid, "done")
	if err != nil {
		return fmt.Errorf("failed to mark task %s as done: %w", uuid, err)
	}
	return nil
}

// MarkDeleted marks a task as deleted (for failed workers).
func MarkDeleted(uuid string) error {
	_, err := runTaskWithInput("yes\n", uuid, "delete")
	if err != nil {
		return fmt.Errorf("failed to delete task %s: %w", uuid, err)
	}
	return nil
}

// GetActiveWorkerTasks returns all pending+active tasks that have a branch UDA (worker tasks).
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

func runTask(args ...string) (string, error) {
	return runTaskWithInput("", args...)
}

func runTaskWithInput(input string, args ...string) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), cmdTimeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, "task", args...)
	if input != "" {
		cmd.Stdin = strings.NewReader(input)
	}

	out, err := cmd.CombinedOutput()
	if ctx.Err() != nil {
		return "", fmt.Errorf("taskwarrior timeout after %s", cmdTimeout)
	}
	if err != nil {
		return "", fmt.Errorf("%w: %s", err, strings.TrimSpace(string(out)))
	}
	return string(out), nil
}

func parseFirstTask(output string) (*Task, error) {
	output = strings.TrimSpace(output)
	if output == "" || output == "[]" {
		return nil, fmt.Errorf("no task found")
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

func isNumeric(s string) bool {
	if len(s) == 0 {
		return false
	}
	for _, c := range s {
		if c < '0' || c > '9' {
			return false
		}
	}
	return true
}

func capitalizeWords(s string) string {
	words := strings.Fields(s)
	for i, w := range words {
		if len(w) > 0 {
			words[i] = strings.ToUpper(w[:1]) + w[1:]
		}
	}
	return strings.Join(words, " ")
}
