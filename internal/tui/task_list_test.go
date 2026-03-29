package tui

import (
	"testing"

	"github.com/tta-lab/ttal-cli/internal/taskwarrior"
)

func TestMoveCursor(t *testing.T) {
	m := Model{
		filtered: []Task{
			{Task: taskwarrior.Task{ID: "1"}},
			{Task: taskwarrior.Task{ID: "2"}},
			{Task: taskwarrior.Task{ID: "3"}},
		},
		cursor: 0,
		offset: 0,
		height: 20,
	}

	m.moveCursor(1)
	if m.cursor != 1 {
		t.Errorf("expected cursor 1, got %d", m.cursor)
	}

	m.moveCursor(1)
	if m.cursor != 2 {
		t.Errorf("expected cursor 2, got %d", m.cursor)
	}

	m.moveCursor(1)
	if m.cursor != 2 {
		t.Errorf("expected cursor 2 (clamped), got %d", m.cursor)
	}

	m.cursor = 2
	m.moveCursor(-1)
	if m.cursor != 1 {
		t.Errorf("expected cursor 1, got %d", m.cursor)
	}

	m.moveCursor(-5)
	if m.cursor != 0 {
		t.Errorf("expected cursor 0 (clamped), got %d", m.cursor)
	}
}

func TestEnsureCursorVisible(t *testing.T) {
	m := Model{
		filtered: []Task{
			{Task: taskwarrior.Task{ID: "1"}},
			{Task: taskwarrior.Task{ID: "2"}},
			{Task: taskwarrior.Task{ID: "3"}},
			{Task: taskwarrior.Task{ID: "4"}},
			{Task: taskwarrior.Task{ID: "5"}},
			{Task: taskwarrior.Task{ID: "6"}},
			{Task: taskwarrior.Task{ID: "7"}},
			{Task: taskwarrior.Task{ID: "8"}},
		},
		cursor: 0,
		offset: 0,
		height: 10,
	}

	m.cursor = 5
	m.offset = 0
	m.ensureCursorVisible()
	if m.offset != 0 {
		t.Errorf("expected offset 0 (cursor 5 visible in 6-row viewport), got %d", m.offset)
	}

	m.cursor = 0
	m.offset = 5
	m.ensureCursorVisible()
	if m.offset != 0 {
		t.Errorf("expected offset 0, got %d", m.offset)
	}

	m.cursor = 3
	m.offset = 0
	m.ensureCursorVisible()
	if m.offset != 0 {
		t.Errorf("expected offset 0 (cursor in view), got %d", m.offset)
	}
}
