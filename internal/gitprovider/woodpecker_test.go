package gitprovider

import (
	"strings"
	"testing"

	woodpecker "go.woodpecker-ci.org/woodpecker/v3/woodpecker-go/woodpecker"
)

func TestIsWoodpeckerContext(t *testing.T) {
	tests := []struct {
		context  string
		expected bool
	}{
		{"ci/woodpecker/pr/lint", true},
		{"ci/woodpecker/push/build", true},
		{"ci/jenkins", false},
		{"lint", false},
		{"", false},
	}
	for _, tt := range tests {
		if got := IsWoodpeckerContext(tt.context); got != tt.expected {
			t.Errorf("IsWoodpeckerContext(%q) = %v, want %v", tt.context, got, tt.expected)
		}
	}
}

func TestFormatWoodpeckerLogs_FiltersNonOutputTypes(t *testing.T) {
	entries := []*woodpecker.LogEntry{
		{Data: []byte("stdout line"), Type: woodpecker.LogEntryStdout},
		{Data: []byte("stderr line"), Type: woodpecker.LogEntryStderr},
		{Data: []byte("exit code"), Type: woodpecker.LogEntryExitCode},
	}
	got := formatWoodpeckerLogs(entries, 50)
	if strings.Contains(got, "exit code") {
		t.Error("expected ExitCode entries to be filtered out")
	}
	if !strings.Contains(got, "stdout line") {
		t.Error("expected stdout lines to be included")
	}
	if !strings.Contains(got, "stderr line") {
		t.Error("expected stderr lines to be included")
	}
}

func TestFormatWoodpeckerLogs_TailsTruncation(t *testing.T) {
	entries := make([]*woodpecker.LogEntry, 10)
	for i := range entries {
		entries[i] = &woodpecker.LogEntry{
			Data: []byte(strings.Repeat("x", 1)),
			Type: woodpecker.LogEntryStdout,
		}
	}
	// Keep only last 3
	got := formatWoodpeckerLogs(entries, 3)
	lines := strings.Split(got, "\n")
	if len(lines) != 3 {
		t.Errorf("expected 3 lines after truncation, got %d", len(lines))
	}
}

func TestFormatWoodpeckerLogs_EmptyEntries(t *testing.T) {
	got := formatWoodpeckerLogs([]*woodpecker.LogEntry{}, 50)
	if got != "" {
		t.Errorf("expected empty string for empty entries, got %q", got)
	}
}
func TestFormatWoodpeckerLogs_NegativeMaxLines(t *testing.T) {
	entries := []*woodpecker.LogEntry{
		{Data: []byte("a"), Type: woodpecker.LogEntryStdout},
		{Data: []byte("b"), Type: woodpecker.LogEntryStdout},
	}
	got := formatWoodpeckerLogs(entries, -1)
	if got != "" {
		t.Errorf("expected empty string for negative maxLines, got %q", got)
	}
}

func TestFormatWoodpeckerLogs_ZeroMaxLines(t *testing.T) {
	entries := []*woodpecker.LogEntry{
		{Data: []byte("a"), Type: woodpecker.LogEntryStdout},
	}
	got := formatWoodpeckerLogs(entries, 0)
	if got != "" {
		t.Errorf("expected empty string for zero maxLines, got %q", got)
	}
}

func TestFormatWoodpeckerLogs_ExactlyMaxLines(t *testing.T) {
	entries := make([]*woodpecker.LogEntry, 5)
	for i := range entries {
		entries[i] = &woodpecker.LogEntry{
			Data: []byte(strings.Repeat("x", 1)),
			Type: woodpecker.LogEntryStdout,
		}
	}
	got := formatWoodpeckerLogs(entries, 5)
	lines := strings.Split(got, "\n")
	if len(lines) != 5 {
		t.Errorf("expected 5 lines when maxLines equals count, got %d", len(lines))
	}
}

func TestFormatWoodpeckerLogs_MoreThanEntries(t *testing.T) {
	entries := []*woodpecker.LogEntry{
		{Data: []byte("a"), Type: woodpecker.LogEntryStdout},
	}
	got := formatWoodpeckerLogs(entries, 100)
	if strings.Count(got, "\n") != 0 {
		t.Errorf("expected 1 line for 1 entry with maxLines=100, got %q", got)
	}
}
