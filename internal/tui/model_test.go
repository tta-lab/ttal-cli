package tui

import (
	"testing"

	"github.com/tta-lab/ttal-cli/internal/taskwarrior"
)

func TestEnsureCursorVisible_NegativeOffset(t *testing.T) {
	tasks := make([]Task, 5)
	for i := range tasks {
		tasks[i] = Task{
			Task: taskwarrior.Task{
				ID:          i + 1,
				Description: "task",
			},
		}
	}
	m := Model{
		filtered: tasks,
		cursor:   0,
		offset:   -2,
		height:   10,
	}

	m.ensureCursorVisible()

	if m.offset < 0 {
		t.Errorf("offset should not be negative, got %d", m.offset)
	}
}

func TestEnsureCursorVisible_ScrollDown(t *testing.T) {
	tasks := make([]Task, 5)
	for i := range tasks {
		tasks[i] = Task{
			Task: taskwarrior.Task{
				ID:          i + 1,
				Description: "task",
			},
		}
	}
	m := Model{
		filtered: tasks,
		cursor:   4,
		offset:   0,
		height:   5,
	}

	m.ensureCursorVisible()

	if m.offset != 4 {
		t.Errorf("expected offset 4, got %d", m.offset)
	}
}

func TestEnsureCursorVisible_NoPanicOnEmpty(t *testing.T) {
	m := Model{
		filtered: []Task{},
		cursor:   0,
		offset:   0,
		height:   10,
	}

	defer func() {
		if r := recover(); r != nil {
			t.Errorf("ensureCursorVisible panicked with empty tasks: %v", r)
		}
	}()

	m.ensureCursorVisible()
}
