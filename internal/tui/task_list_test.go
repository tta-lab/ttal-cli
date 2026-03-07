package tui

import (
	"testing"

	"charm.land/bubbles/v2/table"
	"github.com/tta-lab/ttal-cli/internal/taskwarrior"
)

func TestTableModel_Integration(t *testing.T) {
	task := Task{
		Task: taskwarrior.Task{
			ID:          1,
			Description: "test task",
		},
		Priority: "H",
	}
	m := Model{
		filtered: []Task{task},
		cursor:   0,
	}

	cols := []table.Column{
		{Title: "ID", Width: 5},
		{Title: "P", Width: 2},
		{Title: "Description", Width: 0},
	}
	m.taskTable = table.New(
		table.WithColumns(cols),
		table.WithRows([]table.Row{
			{"1", "H", "test task"},
			{"2", "M", "another task"},
		}),
		table.WithFocused(true),
	)

	if m.taskTable.Cursor() != 0 {
		t.Error("expected cursor to start at 0")
	}

	m.taskTable.MoveDown(1)
	if m.taskTable.Cursor() != 1 {
		t.Error("expected cursor to move to 1")
	}

	m.taskTable.SetCursor(0)
	if m.taskTable.Cursor() != 0 {
		t.Error("expected cursor to be set to 0")
	}
}
