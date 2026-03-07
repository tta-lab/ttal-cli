package tui

import (
	"testing"

	"github.com/tta-lab/ttal-cli/internal/taskwarrior"
)

func TestGetCellStyle_NegativeIndex(t *testing.T) {
	task := Task{
		Task: taskwarrior.Task{
			ID:          1,
			Description: "test task",
		},
	}
	m := Model{
		filtered: []Task{task},
		offset:   -2,
		cursor:   1,
	}

	defer func() {
		if r := recover(); r != nil {
			t.Errorf("getCellStyle should not panic with negative index: %v", r)
		}
	}()

	style := m.getCellStyle(1, 0)
	if style.GetBold() {
		t.Error("expected empty style for negative index, got bold style")
	}
}

func TestGetCellStyle_ValidIndex(t *testing.T) {
	task := Task{
		Task: taskwarrior.Task{
			ID:          1,
			Description: "test task",
		},
		Priority: "H",
	}
	m := Model{
		filtered: []Task{task},
		offset:   0,
		cursor:   0,
	}

	style := m.getCellStyle(1, 0)
	if !style.GetBold() {
		t.Error("expected bold style for header row (row=0)")
	}
}

func TestGetCellStyle_OutOfBounds(t *testing.T) {
	task := Task{
		Task: taskwarrior.Task{
			ID:          1,
			Description: "test task",
		},
	}
	m := Model{
		filtered: []Task{task},
		offset:   0,
		cursor:   0,
	}

	defer func() {
		if r := recover(); r != nil {
			t.Errorf("getCellStyle panicked with out of bounds index: %v", r)
		}
	}()

	_ = m.getCellStyle(10, 0)
}

func TestGetCellStyle_HeaderRow(t *testing.T) {
	m := Model{
		filtered: []Task{},
		offset:   0,
		cursor:   0,
	}

	style := m.getCellStyle(0, 0)
	if !style.GetBold() {
		t.Error("expected bold style for header row")
	}
}
