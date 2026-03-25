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
		b.WriteString(styleDim.Render("  " + m.loadingSpinner.View() + " Loading tasks..."))
		return m.padToHeight(b.String())
	}

	if len(m.filtered) == 0 {
		b.WriteString(styleDim.Render("  No tasks found."))
		return m.padToHeight(b.String())
	}

	// Column widths
	colUUID := 10
	colPri := 3
	colAge := 6
	colProject := 12
	colTags := 12
	colDesc := m.width - colUUID - colPri - colAge - colProject - colTags - 10
	if colDesc < 20 {
		colDesc = 20
	}

	// Header
	header := styleDim.Render(
		fmt.Sprintf(" %-*s %-*s %-*s %-*s %-*s %s",
			colUUID, "ID", colPri, "P",
			colAge, "Age", colProject, "Project", colTags, "Tags", "Description"))
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

		uuid := t.ShortUUID()
		pri := t.Priority
		if pri == "" {
			pri = "-"
		}
		age := t.Age()
		if age == "" {
			age = "-"
		}
		proj := truncate(t.Project, colProject)
		tags := truncate(strings.Join(t.Tags, " "), colTags)
		desc := truncate(t.Description, colDesc)

		line := fmt.Sprintf(" %-*s %-*s %-*s %-*s %-*s %s",
			colUUID, uuid, colPri, pri,
			colAge, age, colProject, proj, colTags, tags, desc)

		if selected {
			line = styleSelected.Render(line)
		} else if t.IsToday() && m.filter == filterPending {
			// Today-or-overdue scheduled task: blue background — only in pending view.
			// Uses plain line because lipgloss Width() padding emits ANSI reset sequences
			// that clear the outer background when cells are styled individually.
			line = styleToday.Render(line)
		} else {
			styledUUID := lipgloss.NewStyle().Width(colUUID).Render(styleDim.Render(uuid))
			styledPri := lipgloss.NewStyle().Width(colPri).Render(priorityStyle(t.Priority).Render(pri))
			styledAge := lipgloss.NewStyle().Width(colAge).Render(styleDim.Render(age))
			styledProj := lipgloss.NewStyle().Width(colProject).Render(proj)
			styledTags := lipgloss.NewStyle().Width(colTags).Render(styleTag.Render(tags))

			line = " " + styledUUID + " " + styledPri + " " +
				styledAge + " " + styledProj + " " + styledTags + " " + desc
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
		parts = append(parts, fmt.Sprintf("Search: %s", m.searchInput.Value()))
	} else if m.statusMsg != "" {
		parts = append(parts, m.statusMsg)
	}

	if m.searchInput.Value() != "" && m.state != stateSearch {
		parts = append(parts, styleDim.Render("[/"+m.searchInput.Value()+"]"))
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
