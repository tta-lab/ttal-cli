package flicktask

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
	"time"
)

var hexIDPattern = regexp.MustCompile(`^[0-9a-f]{8}$`)

// HexIDPattern finds a flicknote hex ID (8+ lowercase hex chars) anywhere in an annotation.
var HexIDPattern = regexp.MustCompile(`\b([a-f0-9]{8,})\b`)

var hexIDLongPattern = regexp.MustCompile(`^[a-f0-9]{8,}$`)

// IsHexID returns true if s looks like a bare flicknote/UUID hex prefix (8+ hex chars).
func IsHexID(s string) bool {
	return hexIDLongPattern.MatchString(s)
}

const cmdTimeout = 5 * time.Second

// flicknoteTimeout is longer than cmdTimeout because flicknote may need to
// fetch from a remote API on first access (cache miss).
const flicknoteTimeout = 10 * time.Second

// UserError is an error with a user-facing message intended for CLI display.
type UserError struct {
	Msg string
}

func (e *UserError) Error() string { return e.Msg }

func userError(format string, args ...any) error {
	return &UserError{Msg: fmt.Sprintf(format, args...)}
}

// Annotation represents a flicktask annotation.
type Annotation struct {
	Description string `json:"description"`
	Entry       string `json:"entry,omitempty"`
}

// Task represents a flicktask task with worker UDAs.
type Task struct {
	UUID        string       `json:"uuid"`
	Description string       `json:"description"`
	Project     string       `json:"project,omitempty"`
	Status      string       `json:"status"`
	Tags        []string     `json:"tags,omitempty"`
	Annotations []Annotation `json:"annotations,omitempty"`
	Start       string       `json:"start,omitempty"`
	End         string       `json:"end,omitempty"`
	Entry       string       `json:"entry,omitempty"`
	Modified    string       `json:"modified,omitempty"`
	Scheduled   string       `json:"scheduled,omitempty"`
	Due         string       `json:"due,omitempty"`
	Priority    string       `json:"priority,omitempty"`
	Branch      string       `json:"branch,omitempty"`
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
func (t *Task) SessionName() string {
	prefix := "w-" + t.SessionID() + "-"

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
func slugify(input string, maxLen int) string {
	s := strings.ToLower(strings.TrimSpace(input))

	for _, prefix := range []string{
		"feat/", "fix/", "worker/", "chore/", "refactor/", "docs/",
		"feat:", "fix:", "chore:", "refactor:", "docs:",
		"feat(", "fix(", "chore(", "refactor(",
	} {
		s = strings.TrimPrefix(s, prefix)
	}

	if idx := strings.Index(s, "):"); idx != -1 {
		s = strings.TrimSpace(s[idx+2:])
	}

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

// ValidateID checks that s is a valid flicktask ID (8-char hex).
func ValidateID(s string) error {
	s = strings.TrimSpace(s)

	if s == "" {
		return fmt.Errorf("task ID is required")
	}

	if strings.HasPrefix(s, "#") {
		remaining := s[1:]
		if isNumeric(remaining) {
			return userError("# prefix format is not supported\n\n"+
				"  You provided: %s\n\n"+
				"  Use the bare 8-char hex ID instead:\n\n"+
				"  # Get ID for task:\n"+
				"  flicktask list | grep <description>", s)
		}
		return userError("# prefix format is not supported\n\n"+
			"  You provided: %s\n\n"+
			"  Remove the # prefix:\n"+
			"  ttal task execute %s", s, remaining)
	}

	if isNumeric(s) {
		return userError("numeric task IDs are not supported\n\n"+
			"  You provided: %s\n\n"+
			"  Use the permanent 8-char hex ID instead:\n\n"+
			"  # Find task ID:\n"+
			"  flicktask list\n\n"+
			"  # Then use the hex ID:\n"+
			"  ttal task execute <hex-id>", s)
	}

	if !hexIDPattern.MatchString(s) {
		// Also accept full UUIDs
		uuidPattern := regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$`)
		if !uuidPattern.MatchString(s) {
			return userError("only 8-char hex IDs or full UUIDs are supported\n\n"+
				"  You provided: %s\n\n"+
				"  Provide an 8-char hex ID.\n"+
				"  Example: flicktask list", s)
		}
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

func runFlicktask(args ...string) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), cmdTimeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, "flicktask", args...)

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()
	if ctx.Err() != nil {
		return "", fmt.Errorf("flicktask timeout after %s", cmdTimeout)
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
