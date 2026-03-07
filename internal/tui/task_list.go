package tui

import (
	"fmt"
	"strings"

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

	// Column widths
	idWidth := 5
	uuidWidth := 8
	priWidth := 2
	ageWidth := 5
	projWidth := 12
	tagWidth := 12

	// Render visible rows
	visible := m.visibleRows()
	end := m.offset + visible
	if end > len(m.filtered) {
		end = len(m.filtered)
	}

	for i := m.offset; i < end; i++ {
		t := &m.filtered[i]
		pri := t.Priority
		if pri == "" {
			pri = "-"
		}
		age := t.Age()
		if age == "" {
			age = "-"
		}

		rowStyle := styleRow
		if i == m.cursor {
			rowStyle = styleSelected
		}

		row := fmt.Sprintf(" %-*s %-*s %-*s %-*s %-*s %-*s %s",
			idWidth, fmt.Sprintf("%d", t.ID),
			uuidWidth, t.ShortUUID(),
			priWidth, pri,
			ageWidth, age,
			projWidth, truncate(t.Project, projWidth),
			tagWidth, truncate(strings.Join(t.Tags, " "), tagWidth),
			t.Description,
		)
		b.WriteString(rowStyle.Render(row))
		b.WriteString("\n")
	}

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
