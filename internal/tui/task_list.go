package tui

import (
	"fmt"
	"strings"

	"charm.land/bubbles/v2/table"
	"charm.land/lipgloss/v2"
)

func (m Model) viewTaskList() string {
	var b strings.Builder

	// Title bar
	teamLabel := m.teamName
	if teamLabel == "" {
		teamLabel = "default"
	}
	title := styleTitle.Render(fmt.Sprintf(" TTal [%s]  [%s]", teamLabel, m.filter))
	count := styleDim.Render(fmt.Sprintf("  %d tasks", len(m.filtered)))
	b.WriteString(title + count)
	b.WriteString("\n")

	if m.loading {
		b.WriteString(styleDim.Render("  Loading tasks..."))
		return m.padToHeight(b.String())
	}

	if len(m.filtered) == 0 {
		b.WriteString(styleDim.Render("  No tasks found."))
		return m.padToHeight(b.String())
	}

	// Build rows for table
	rows := make([]table.Row, 0, len(m.filtered))
	for i := range m.filtered {
		t := &m.filtered[i]
		pri := t.Priority
		if pri == "" {
			pri = "-"
		}
		age := t.Age()
		if age == "" {
			age = "-"
		}
		rows = append(rows, table.Row{
			fmt.Sprintf("%d", t.ID),
			t.ShortUUID(),
			pri,
			age,
			truncate(t.Project, 12),
			truncate(strings.Join(t.Tags, " "), 12),
			t.Description,
		})
	}

	// Update table model
	m.taskTable.SetRows(rows)
	m.taskTable.SetWidth(m.width)
	m.taskTable.SetHeight(m.visibleRows())

	// Set cursor to match our model's cursor
	m.taskTable.SetCursor(m.cursor)

	b.WriteString(m.taskTable.View())
	b.WriteString("\n")

	result := m.padToHeight(b.String())

	// Status bar at bottom
	result += m.viewStatusBar()

	return result
}

func (m Model) viewStatusBar() string {
	var parts []string

	if m.state == stateSearch {
		parts = append(parts, fmt.Sprintf("Search: %s_", m.searchStr))
	} else if m.statusMsg != "" {
		parts = append(parts, m.statusMsg)
	}

	if m.searchStr != "" && m.state != stateSearch {
		parts = append(parts, styleDim.Render("[/"+m.searchStr+"]"))
	}

	if t := m.selectedTask(); t != nil {
		parts = append(parts, styleDim.Render(t.UUID))
	}

	right := styleHelp.Render("? help  f filter  / search  q quit")

	left := styleStatusBar.Render(strings.Join(parts, "  "))

	gap := m.width - lipgloss.Width(left) - lipgloss.Width(right)
	if gap < 1 {
		gap = 1
	}

	return left + strings.Repeat(" ", gap) + right
}

func (m Model) padToHeight(content string) string {
	lines := strings.Count(content, "\n")
	// Reserve 1 line for status bar
	target := m.height - 1
	if lines < target {
		content += strings.Repeat("\n", target-lines)
	}
	return content
}

func truncate(s string, maxLen int) string {
	runes := []rune(s)
	if len(runes) <= maxLen {
		return s
	}
	if maxLen <= 1 {
		return string(runes[:maxLen])
	}
	return string(runes[:maxLen-1]) + "~"
}
