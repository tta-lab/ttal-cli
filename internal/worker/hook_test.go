package worker

import (
	"testing"
)

func TestHookTask_SetTag_NoDuplicate(t *testing.T) {
	task := hookTask{"tags": []any{"bugfix", "kestrel"}}
	task.SetTag("kestrel") // should not add duplicate
	if len(task.Tags()) != 2 {
		t.Errorf("expected 2 tags, got %d", len(task.Tags()))
	}
}

func TestHookTask_SetTag_NilTags(t *testing.T) {
	task := hookTask{"uuid": "test"}
	task.SetTag("kestrel")
	tags := task.Tags()
	if len(tags) != 1 || tags[0] != "kestrel" {
		t.Errorf("expected [kestrel], got %v", tags)
	}
}

func TestHookTask_SetStart_Format(t *testing.T) {
	task := hookTask{}
	task.SetStart()
	start := task.Start()
	// Taskwarrior format: 20060102T150405Z
	if len(start) != 16 || start[8] != 'T' || start[15] != 'Z' {
		t.Errorf("unexpected start format: %s", start)
	}
}
