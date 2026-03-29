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
	UUID        string       `json:"uuid"`
	Description string       `json:"description"`
	Project     string       `json:"project,omitempty"`
	Status      string       `json:"status"`
	Urgency     float64      `json:"urgency"`
	Tags        []string     `json:"tags,omitempty"`
	Annotations []Annotation `json:"annotations,omitempty"`
	Start       string       `json:"start,omitempty"`
	Modified    string       `json:"modified,omitempty"`
	Priority    string       `json:"priority,omitempty"`
	Scheduled   string       `json:"scheduled,omitempty"`
	Due         string       `json:"due,omitempty"`
	Entry       string       `json:"entry,omitempty"`
	End         string       `json:"end,omitempty"`
	PRID        string       `json:"pr_id,omitempty"`
	Spawner     string       `json:"spawner,omitempty"`
}

// HexID returns the first 8 hex characters of the task UUID.
func (t *Task) HexID() string {
	if len(t.UUID) >= 8 {
		return t.UUID[:8]
	}
	return t.UUID
}

// SessionName returns a human-readable session name: w-{uuid[:8]}-{slug}.
// Slug is always derived from the task description, which is immutable and
// stable across the task lifetime. This ensures open/attach commands always
// find the session regardless of when they run relative to UDA writes.
//
// Worker sessions use this format to be identifiable at a glance:
//
//	w-e9d4b7c1-fix-auth
//	w-a3f29bc0-add-doctor
//
// This is distinct from agent sessions which use "ttal-<team>-<agent>".
func (t *Task) SessionName() string {
	prefix := "w-" + t.HexID() + "-" // "w-e9d4b7c1-" = 11 chars

	slug := slugify(t.Description, 64)
	if slug == "" {
		return "w-" + t.HexID()
	}

	return prefix + slug
}

// IsActive returns true if the task is currently started.
func (t *Task) IsActive() bool {
	return t.Start != ""
}

// IsToday returns true if the task is scheduled for today or earlier.
func (t *Task) IsToday() bool {
	if t.Scheduled == "" {
		return false
	}
	parsed, err := ParseTaskDate(t.Scheduled)
	if err != nil {
		return false
	}
	today := time.Now().Truncate(24 * time.Hour)
	return !parsed.After(today)
}

// Age returns a human-readable age string based on the entry date.
func (t *Task) Age() string {
	if t.Entry == "" {
		return ""
	}
	parsed, err := time.Parse("20060102T150405Z", t.Entry)
	if err != nil {
		return "?"
	}
	return formatAge(time.Since(parsed))
}

func formatAge(d time.Duration) string {
	if d < time.Hour {
		return fmt.Sprintf("%dm", int(d.Minutes()))
	}
	if d < 24*time.Hour {
		return fmt.Sprintf("%dh", int(d.Hours()))
	}
	if d < 30*24*time.Hour {
		return fmt.Sprintf("%dd", int(d.Hours()/24))
	}
	return fmt.Sprintf("%dmo", int(d.Hours()/24/30))
}

// ParseTaskDate parses taskwarrior date formats (ISO 8601 with T and Z).
func ParseTaskDate(s string) (time.Time, error) {
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

// ExtractHexID extracts the UUID[:8] from a session name.
// Handles w-UUID[:8]-slug (worker) and bare UUID[:8].
func ExtractHexID(sessionName string) string {
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

// HasTag returns true if tags contains the given tag.
func HasTag(tags []string, tag string) bool {
	for _, t := range tags {
		if t == tag {
			return true
		}
	}
	return false
}

// IsLGTMTag reports whether tag is a stage-specific lgtm tag (ends with "_lgtm").
func IsLGTMTag(tag string) bool {
	return strings.HasSuffix(tag, "_lgtm")
}

// HasAnyLGTMTag returns true if any tag in the slice is a stage lgtm tag.
func HasAnyLGTMTag(tags []string) bool {
	for _, t := range tags {
		if IsLGTMTag(t) {
			return true
		}
	}
	return false
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
			"  ttal go %s", s, remaining)
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
			"  ttal go <uuid>", s, s, s)
	}

	if !uuidPattern.MatchString(s) && !uuidPrefixPattern.MatchString(s) {
		return userError("only UUIDs are supported\n\n"+
			"  You provided: %s\n\n"+
			"  Provide a full UUID or 8-char prefix.\n"+
			"  Example: task export | jq -r '.[0].uuid'", s)
	}

	return nil
}

// VerifyRequiredUDAs checks that required UDAs are configured in taskwarrior.
func VerifyRequiredUDAs() error {
	out, err := runTask("show")
	if err != nil {
		return &UserError{Msg: fmt.Sprintf("could not verify UDA configuration: %v\n\n"+
			"  This prevents creating orphaned sessions that aren't tracked.", err)}
	}

	required := []string{"pr_id", "spawner"}
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
}

// ParsePRID parses a pr_id UDA value. Strips legacy ":lgtm" suffix for backward compat.
func ParsePRID(raw string) (PRIDInfo, error) {
	if raw == "" {
		return PRIDInfo{}, fmt.Errorf("empty pr_id")
	}
	clean := strings.TrimSuffix(raw, ":lgtm")
	index, err := strconv.ParseInt(clean, 10, 64)
	if err != nil {
		return PRIDInfo{}, fmt.Errorf("invalid pr_id %q: %w", raw, err)
	}
	return PRIDInfo{Index: index}, nil
}

// SetPRID sets the pr_id UDA on a task.
func SetPRID(uuid, prID string) error {
	_, err := runTask(uuid, "modify", fmt.Sprintf("pr_id:%s", prID))
	if err != nil {
		return fmt.Errorf("failed to set pr_id on task %s: %w", uuid, err)
	}
	return nil
}
