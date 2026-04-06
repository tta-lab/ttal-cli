package tui

import (
	"testing"

	"github.com/charmbracelet/x/ansi"
)

func TestRenderRowTruncatesEmojiByVisualWidth(t *testing.T) {
	m := Model{
		width:            80,
		height:           20,
		filter:           filterActive,
		agentEmojiByName: map[string]string{"yuki": "🐱"},
		pipelineCfg:      nil, // no pipeline — stage will be empty
	}
	cols := m.buildColumns()

	task := Task{
		UUID:        "aabbccdd-1234-5678-9abc-def012345678",
		Description: "Test task",
		Priority:    "H",
		Project:     "ttal",
		Tags:        []string{"yuki"},
		Start:       "20260101T100000Z", // active
	}
	m.filtered = []Task{task}

	rendered := m.renderRow(0, cols)
	visualWidth := ansi.StringWidth(rendered)
	if visualWidth > m.width {
		t.Errorf("row visual width %d exceeds terminal width %d (emoji agent caused wrapping)", visualWidth, m.width)
	}
}

func TestRenderRowFitsWidth(t *testing.T) {
	tests := []struct {
		name   string
		filter filterMode
		task   Task
	}{
		{
			name:   "standard row with tags",
			filter: filterPending,
			task: Task{
				UUID:        "aabbccdd-1234-5678-9abc-def012345678",
				Description: "A task with a long description that should be truncated properly",
				Priority:    "H",
				Project:     "ttal",
				Tags:        []string{"feature"},
			},
		},
		{
			name:   "active row with emoji agent",
			filter: filterActive,
			task: Task{
				UUID:        "aabbccdd-1234-5678-9abc-def012345678",
				Description: "Active task with emoji agent",
				Priority:    "M",
				Project:     "ttal",
				Tags:        []string{"yuki", "implement"},
				Start:       "20260101T100000Z",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := Model{
				width:            80,
				height:           20,
				filter:           tt.filter,
				agentEmojiByName: map[string]string{"yuki": "🐱"},
			}
			cols := m.buildColumns()
			m.filtered = []Task{tt.task}

			rendered := m.renderRow(0, cols)
			visualWidth := ansi.StringWidth(rendered)
			if visualWidth > m.width {
				t.Errorf("row visual width %d exceeds terminal width %d", visualWidth, m.width)
			}
		})
	}
}

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
