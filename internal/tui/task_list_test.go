package tui

import (
	"testing"

	"github.com/tta-lab/ttal-cli/internal/taskwarrior"
)

func TestMoveCursor(t *testing.T) {
	m := Model{
		filtered: []Task{
			{Task: taskwarrior.Task{ID: 1}},
			{Task: taskwarrior.Task{ID: 2}},
			{Task: taskwarrior.Task{ID: 3}},
		},
		cursor: 0,
		offset: 0,
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
