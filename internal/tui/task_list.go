package tui

import (
	"fmt"
	"strings"

	"charm.land/lipgloss/v2"
)

func (m Model) viewTaskList() string {
	var b strings.Builder

	// Title bar
	title := styleTitle.Render(fmt.Sprintf(" ttal tui  [%s]", m.filter))
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
	colID := 4
	colUUID := 10
	colPri := 3
	colProject := 14
	colTags := 14
	colDesc := m.width - colID - colUUID - colPri - colProject - colTags - 8
	if colDesc < 20 {
		colDesc = 20
	}

	// Header
	header := styleDim.Render(
		fmt.Sprintf(" %-*s %-*s %-*s %-*s %-*s %s",
			colID, "ID", colUUID, "UUID", colPri, "P",
			colProject, "Project", colTags, "Tags", "Description"))
	b.WriteString(header)
	b.WriteString("\n")

	// Visible rows
	visible := m.visibleRows()
	end := m.offset + visible
	if end > len(m.filtered) {
		end = len(m.filtered)
	}

	for i := m.offset; i < end; i++ {
		t := &m.filtered[i]
		selected := i == m.cursor

		id := fmt.Sprintf("%d", t.ID)
		uuid := t.ShortUUID()
		pri := t.Priority
		if pri == "" {
			pri = "-"
		}
		proj := truncate(t.Project, colProject)
		tags := truncate(strings.Join(t.Tags, " "), colTags)
		desc := truncate(t.Description, colDesc)

		line := fmt.Sprintf(" %-*s %-*s %-*s %-*s %-*s %s",
			colID, id, colUUID, uuid, colPri, pri,
			colProject, proj, colTags, tags, desc)

		if selected {
			line = styleSelected.Render(line)
		} else {
			// Style priority column
			styledPri := priorityStyle(t.Priority).Render(fmt.Sprintf("%-*s", colPri, pri))
			line = fmt.Sprintf(" %s %s %s %-*s %-*s %s",
				styleDim.Render(fmt.Sprintf("%-*s", colID, id)),
				styleDim.Render(fmt.Sprintf("%-*s", colUUID, uuid)),
				styledPri,
				colProject, proj,
				colTags, styleTag.Render(tags),
				desc)

			if t.Start != "" {
				line = lipgloss.NewStyle().Foreground(colorCyan).Render(line)
			}
		}

		b.WriteString(line)
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
