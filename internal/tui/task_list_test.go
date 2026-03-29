package tui

import "testing"

func TestMoveCursor(t *testing.T) {
	m := Model{
		filtered: []Task{
			{UUID: "aa000001"},
			{UUID: "bb000002"},
			{UUID: "cc000003"},
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
			{UUID: "aa000001"},
			{UUID: "bb000002"},
			{UUID: "cc000003"},
			{UUID: "dd000004"},
			{UUID: "ee000005"},
			{UUID: "ff000006"},
			{UUID: "a1000007"},
			{UUID: "b2000008"},
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
