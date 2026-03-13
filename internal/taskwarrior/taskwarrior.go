package taskwarrior

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"
)

var uuidPattern = regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$`)
var uuidPrefixPattern = regexp.MustCompile(`^[0-9a-f]{8}$`)
var uuidFindPattern = regexp.MustCompile(`[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}`)

// HexIDPattern finds a flicknote hex ID (8+ lowercase hex chars) anywhere in an annotation.
// Matches bare IDs ("e8fd0fe0"), prefixed ("Plan: e8fd0fe0"), or multi-word
// ("Plan: flicknote b7b61e89"). If the ID doesn't exist in flicknote, ReadFlicknoteJSON
// returns nil and the annotation is suppressed from the prompt.
var HexIDPattern = regexp.MustCompile(`\b([a-f0-9]{8,})\b`)

// IsHexID returns true if s looks like a bare flicknote/UUID hex prefix (8+ hex chars).
func IsHexID(s string) bool {
	return regexp.MustCompile(`^[a-f0-9]{8,}$`).MatchString(s)
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
	Scheduled   string       `json:"scheduled,omitempty"`
	Branch      string       `json:"branch"`
	ProjectPath string       `json:"project_path"`
	PRID        string       `json:"pr_id,omitempty"`
	Spawner     string       `json:"spawner,omitempty"`
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
			"  ttal task execute %s", s, remaining)
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
			"  ttal task execute <uuid>", s, s, s)
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

// SetSpawner sets the spawner UDA on a task (format: team:agent).
func SetSpawner(uuid, spawner string) error {
	_, err := runTask(uuid, "modify", fmt.Sprintf("spawner:%s", spawner))
	if err != nil {
		return fmt.Errorf("failed to set spawner on task %s: %w", uuid, err)
	}
	return nil
}

// PRIDInfo holds parsed pr_id UDA data.
type PRIDInfo struct {
	Index int64
	LGTM  bool
	Raw   string
}

// ParsePRID parses a pr_id UDA value. Accepts "123" or "123:lgtm".
func ParsePRID(raw string) (PRIDInfo, error) {
	if raw == "" {
		return PRIDInfo{}, fmt.Errorf("empty pr_id")
	}
	info := PRIDInfo{Raw: raw}
	numStr := raw
	if strings.HasSuffix(raw, ":lgtm") {
		info.LGTM = true
		numStr = strings.TrimSuffix(raw, ":lgtm")
	}
	index, err := strconv.ParseInt(numStr, 10, 64)
	if err != nil {
		return PRIDInfo{}, fmt.Errorf("invalid pr_id %q: %w", raw, err)
	}
	info.Index = index
	return info, nil
}

// SetPRLGTM appends :lgtm to the task's pr_id UDA.
// If already has :lgtm, this is a no-op.
func SetPRLGTM(uuid string) error {
	task, err := ExportTask(uuid)
	if err != nil {
		return fmt.Errorf("failed to read task %s: %w", uuid, err)
	}
	if task.PRID == "" {
		return fmt.Errorf("task %s has no pr_id", uuid)
	}
	if strings.HasSuffix(task.PRID, ":lgtm") {
		return nil // already approved
	}
	return SetPRID(uuid, task.PRID+":lgtm")
}

// SetPRID sets the pr_id UDA on a task.
func SetPRID(uuid, prID string) error {
	_, err := runTask(uuid, "modify", fmt.Sprintf("pr_id:%s", prID))
	if err != nil {
		return fmt.Errorf("failed to set pr_id on task %s: %w", uuid, err)
	}
	return nil
}
