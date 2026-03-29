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
		{
			name:   "child row",
			filter: filterPending,
			task: Task{
				UUID:        "bbbbcccc-1234-5678-9abc-def012345678",
				Description: "Child task",
				ParentID:    "aabbccdd-1234-5678-9abc-def012345678",
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
