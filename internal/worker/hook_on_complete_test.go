package worker

import (
	"strings"
	"testing"
)

func TestIsCompletionEvent(t *testing.T) {
	tests := []struct {
		name     string
		original hookTask
		modified hookTask
		want     bool
	}{
		{
			"pending to completed",
			hookTask{"status": "pending"},
			hookTask{"status": "completed"},
			true,
		},
		{
			"pending to pending (no change)",
			hookTask{"status": "pending"},
			hookTask{"status": "pending"},
			false,
		},
		{
			"already completed (no-op)",
			hookTask{"status": "completed"},
			hookTask{"status": "completed"},
			false,
		},
		{
			"deleted to completed",
			hookTask{"status": "deleted"},
			hookTask{"status": "completed"},
			true,
		},
		{
			"started task to completed",
			hookTask{"status": "pending", "start": "20260224T120000Z"},
			hookTask{"status": "completed"},
			true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := isCompletionEvent(tt.original, tt.modified)
			if got != tt.want {
				t.Errorf("isCompletionEvent() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestFormatCompletionMessage(t *testing.T) {
	task := hookTask{
		"uuid":        "abc-123",
		"description": "Fix login bug",
	}
	msg := formatCompletionMessage(task)
	if msg == "" {
		t.Fatal("formatCompletionMessage returned empty")
	}
	if !strings.Contains(msg, "Fix login bug") {
		t.Errorf("message should contain description, got: %s", msg)
	}
	if !strings.Contains(msg, "abc-123") {
		t.Errorf("message should contain uuid, got: %s", msg)
	}
}
