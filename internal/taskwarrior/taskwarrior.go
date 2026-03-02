package taskwarrior

import (
	"bytes"
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
var uuidPrefixPattern = regexp.MustCompile(`^[0-9a-f]{8}$`)

// hexIDPattern matches bare hex IDs (8+ hex chars) used as flicknote note prefixes.
// This intentionally overlaps with uuidPrefixPattern (8-char hex) — in FormatPrompt,
// hex IDs are checked after inlineRefPattern, so UUID-like prefixes trigger a flicknote
// lookup. If the ID doesn't exist in flicknote, readFlicknote returns empty and the
// annotation is treated as plain text.
var hexIDPattern = regexp.MustCompile(`^[a-f0-9]{8,}$`)

// prefixedHexPattern matches annotations like "Plan: e8fd0fe0" or "Research: abcd1234".
var prefixedHexPattern = regexp.MustCompile(`^\w+:\s*([a-f0-9]{8,})$`)

// IsHexID returns true if s looks like a flicknote/UUID hex prefix (8+ hex chars).
func IsHexID(s string) bool {
	return hexIDPattern.MatchString(s)
}

const cmdTimeout = 5 * time.Second

// flicknoteTimeout is longer than cmdTimeout because flicknote may need to
// fetch from a remote API on first access (cache miss).
const flicknoteTimeout = 10 * time.Second

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
	ID          int          `json:"id"`
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

// SessionName returns a human-readable session name: w-{uuid[:8]}-{slug}.
// Slug is derived from branch (preferred) or task description (fallback).
//
// Worker sessions use this format to be identifiable at a glance:
//
//	w-e9d4b7c1-fix-auth
//	w-a3f29bc0-add-doctor
//
// This is distinct from agent sessions which use "ttal-<team>-<agent>".
func (t *Task) SessionName() string {
	prefix := "w-" + t.SessionID() + "-" // "w-e9d4b7c1-" = 11 chars

	source := t.Branch
	if source == "" {
		source = t.Description
	}

	slug := slugify(source, 64)
	if slug == "" {
		return "w-" + t.SessionID()
	}

	return prefix + slug
}

// ExtractSessionID extracts the UUID[:8] from a session name.
// Handles both old format (bare UUID[:8]) and new format (w-UUID[:8]-slug).
func ExtractSessionID(sessionName string) string {
	if strings.HasPrefix(sessionName, "w-") {
		parts := strings.SplitN(sessionName[2:], "-", 2)
		if len(parts) > 0 {
			return parts[0]
		}
	}
	return sessionName
}

// slugify converts a branch name or description into a short URL-safe slug.
// It strips common prefixes (feat/, fix/, worker/, etc.) and truncates to maxLen.
func slugify(input string, maxLen int) string {
	s := strings.ToLower(strings.TrimSpace(input))

	// Strip common branch prefixes
	for _, prefix := range []string{
		"feat/", "fix/", "worker/", "chore/", "refactor/", "docs/",
		"feat:", "fix:", "chore:", "refactor:", "docs:",
		"feat(", "fix(", "chore(", "refactor(",
	} {
		s = strings.TrimPrefix(s, prefix)
	}

	// Strip scope from conventional commits (e.g. "doctor): add foo" → "add foo")
	if idx := strings.Index(s, "):"); idx != -1 {
		s = strings.TrimSpace(s[idx+2:])
	}

	// Replace non-alphanumeric with hyphens
	var b strings.Builder
	prev := '-'
	for _, r := range s {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') {
			b.WriteRune(r)
			prev = r
		} else if prev != '-' {
			b.WriteRune('-')
			prev = '-'
		}
	}

	result := strings.Trim(b.String(), "-")

	// Truncate at word boundary
	if len(result) > maxLen {
		result = result[:maxLen]
		if last := strings.LastIndex(result, "-"); last > maxLen/2 {
			result = result[:last]
		}
	}

	return result
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

// inlineRefPattern matches annotations whose content should be inlined into the prompt.
// Only Plan and Design docs are execution-critical — workers need them to do their job.
var inlineRefPattern = regexp.MustCompile(`(?:Plan|Design):\s*([~\/][\w\/\-\.]+\.md)`)

// referenceRefPattern matches annotations that are shown as-is (path visible, not inlined).
// Research, Doc, Reference, File are context — workers can read them if needed.
var referenceRefPattern = regexp.MustCompile(`(?:Research|Doc|Reference|File):\s*([~\/][\w\/\-\.]+\.md)`)

// rawPathPattern matches annotations that are just a bare file path (no prefix).
// Restricted to .md files for the same reason as inlineRefPattern.
var rawPathPattern = regexp.MustCompile(`^([~\/][\w\/\-\.]+\.md)$`)

// flicknoteNote represents the JSON output of `flicknote get --json`.
type flicknoteNote struct {
	ID      string `json:"id"`
	Title   string `json:"title"`
	Project string `json:"project"`
	Summary string `json:"summary"`
	Content string `json:"content"`
}

// readFlicknoteJSON fetches a note's metadata from flicknote CLI.
// Returns nil if flicknote is not installed, ID not found, or JSON parse fails.
func readFlicknoteJSON(id string) *flicknoteNote {
	ctx, cancel := context.WithTimeout(context.Background(), flicknoteTimeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, "flicknote", "get", "--json", id)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return nil
	}

	var note flicknoteNote
	if err := json.Unmarshal(out, &note); err != nil {
		return nil
	}
	return &note
}

// shouldInlineNote returns true if the note's project indicates it's a plan/design doc.
// Matches both "plan" and "design" for consistency with inlineRefPattern (Plan:/Design:).
func shouldInlineNote(note *flicknoteNote) bool {
	name := strings.ToLower(note.Project)
	return strings.Contains(name, "plan") || strings.Contains(name, "design")
}

// formatFlicknoteContent formats a flicknote note for prompt inlining.
func formatFlicknoteContent(note *flicknoteNote) string {
	var b strings.Builder
	b.WriteString("Title: " + note.Title + "\n")
	if note.Summary != "" {
		b.WriteString("Summary: " + note.Summary + "\n")
	}
	if note.Content != "" {
		b.WriteString("\n" + note.Content)
	}
	return b.String()
}

// docRef represents a reference annotation that should be inlined.
type docRef struct {
	label   string
	refType string // "file", "flicknote_cached"
	id      string // file path or flicknote ID
}

// FormatPrompt formats the task for injection into a worker's Claude prompt.
// Includes description, annotations, and selectively inlined referenced docs.
// File refs use prefix-based logic (Plan:/Design: → inline, Research:/Doc: → don't).
// Hex IDs (bare or prefixed like "Plan: abc123") use project-based logic via flicknote:
// inline if the note's project contains "plan" or "design".
func (t *Task) FormatPrompt() string {
	lines := make([]string, 0, 1+len(t.Annotations))
	lines = append(lines, t.Description)

	refDescs := make(map[string]bool)
	flicknoteCache := make(map[string]string) // keyed by full annotation text (e.g. "Plan: e8fd0fe0")
	var refs []docRef

	for _, ann := range t.Annotations {
		desc := ann.Description

		// 1. Execution-critical prefixed refs: "Plan: ~/path.md", "Design: ~/path.md"
		//    → inline the file content
		if matches := inlineRefPattern.FindAllStringSubmatch(desc, -1); len(matches) > 0 {
			for _, m := range matches {
				refDescs[desc] = true
				refs = append(refs, docRef{label: desc, refType: "file", id: m[1]})
			}
			continue
		}

		// 2. Reference-only prefixed refs: "Research: ~/path.md", "Doc: ~/path.md"
		//    → keep as annotation text (path visible, not inlined)
		if referenceRefPattern.MatchString(desc) {
			continue
		}

		// 3. Hex IDs (bare or prefixed): check flicknote project to decide inlining
		hexID := ""
		if IsHexID(desc) {
			hexID = desc
		} else if m := prefixedHexPattern.FindStringSubmatch(desc); len(m) > 0 {
			hexID = m[1]
		}

		if hexID != "" {
			// Always suppress hex IDs from plain-text output — they're
			// meaningless to the worker if the note can't be resolved.
			refDescs[desc] = true

			note := readFlicknoteJSON(hexID)
			if note != nil && shouldInlineNote(note) {
				flicknoteCache[desc] = formatFlicknoteContent(note)
				refs = append(refs, docRef{label: "FlickNote: " + hexID, refType: "flicknote_cached", id: desc})
			}
			continue
		}

		// 4. Raw file paths: "~/docs/plan.md" or "/absolute/path.md"
		if matches := rawPathPattern.FindStringSubmatch(desc); len(matches) > 0 {
			refDescs[desc] = true
			refs = append(refs, docRef{label: desc, refType: "file", id: matches[1]})
			continue
		}
	}

	// Content annotations (skip refs, they're inlined below)
	for _, ann := range t.Annotations {
		if refDescs[ann.Description] {
			continue
		}
		lines = append(lines, "")
		lines = append(lines, ann.Description)
	}

	result := strings.Join(lines, "\n") + "\n"

	// Inline referenced documents
	if len(refs) > 0 {
		result += "\nReferenced Documentation:\n"
		sep := strings.Repeat("═", 80)
		subSep := strings.Repeat("─", 80)
		for _, ref := range refs {
			result += sep + "\n"
			result += ref.label + "\n"
			result += subSep + "\n"
			switch ref.refType {
			case "file":
				result += readFileRef(ref.id) + "\n"
			case "flicknote_cached":
				result += flicknoteCache[ref.id] + "\n"
			}
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

	// Reject short numeric IDs (taskwarrior task numbers like "42", "123")
	// but allow 8-char hex strings (UUID prefixes like "95502130")
	if isNumeric(s) && len(s) < 8 {
		return userError("numeric task IDs are no longer supported\n\n"+
			"  You provided: %s\n\n"+
			"  Numeric IDs are unstable (they change when tasks complete).\n"+
			"  Use the permanent UUID instead:\n\n"+
			"  # Get UUID for task #%s:\n"+
			"  task %s export | jq -r '.[0].uuid'\n\n"+
			"  # Then use the UUID:\n"+
			"  ttal worker spawn --task <uuid> ...", s, s, s)
	}

	if !uuidPattern.MatchString(s) && !uuidPrefixPattern.MatchString(s) {
		return userError("only UUIDs are supported\n\n"+
			"  You provided: %s\n\n"+
			"  Provide a full UUID or 8-char prefix.\n"+
			"  Example: task export | jq -r '.[0].uuid'", s)
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

// FindTasks searches for tasks matching any of the given keywords (OR logic).
// status filters by task status (e.g. "pending", "completed"). Empty means no filter.
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

// StartTask marks a task as started so it appears in +ACTIVE.
func StartTask(uuid string) error {
	_, err := runTask(uuid, "start")
	if err != nil {
		return fmt.Errorf("failed to start task %s: %w", uuid, err)
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

	cmd := CommandContext(ctx, args...)
	if input != "" {
		cmd.Stdin = strings.NewReader(input)
	}

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()
	if ctx.Err() != nil {
		return "", fmt.Errorf("taskwarrior timeout after %s", cmdTimeout)
	}
	if err != nil {
		errMsg := strings.TrimSpace(stderr.String())
		if errMsg == "" {
			errMsg = strings.TrimSpace(stdout.String())
		}
		return "", fmt.Errorf("%w: %s", err, errMsg)
	}
	return stdout.String(), nil
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
