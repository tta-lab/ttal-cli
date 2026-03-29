package tui

import (
	"fmt"
	"strings"

	"charm.land/lipgloss/v2"
	"github.com/tta-lab/ttal-cli/internal/pipeline"
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
	colUUID := 9 // hex ID is 8 chars + 1 padding
	colPri := 2  // single char (H/M/L/-)
	colAge := 5  // "3mo" is max 3 chars
	colProject := 10
	colTags := 10

	showActiveColumns := m.filter == filterActive
	colAgent := 0
	colStage := 0
	if showActiveColumns {
		colAgent = 10 // "🧭 mira" = ~8 chars
		colStage = 12 // "💡 Brainstorm" = ~12 chars
		colTags = 0   // hide tags in active view
	}

	overhead := colUUID + colPri + colAge + colProject + 7 // 7 = leading space + 6 separators
	if showActiveColumns {
		overhead += colAgent + colStage + 2 // +2 for extra separators
	} else {
		overhead += colTags
	}
	colDesc := m.width - overhead
	if colDesc < 20 {
		colDesc = 20
	}

	// Header
	var header string
	if showActiveColumns {
		header = styleDim.Render(
			fmt.Sprintf(" %-*s %-*s %-*s %-*s %-*s %-*s %s",
				colUUID, "ID", colPri, "P",
				colAge, "Age", colProject, "Project",
				colAgent, "Agent", colStage, "Stage", "Description"))
	} else {
		header = styleDim.Render(
			fmt.Sprintf(" %-*s %-*s %-*s %-*s %-*s %s",
				colUUID, "ID", colPri, "P",
				colAge, "Age", colProject, "Project", colTags, "Tags", "Description"))
	}
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
		isChild := t.IsSubtask()

		uuid := t.HexID()
		pri := t.Priority
		if pri == "" {
			pri = "-"
		}
		age := t.Age()
		if age == "" {
			age = "-"
		}
		proj := truncate(t.Project, colProject)
		tags := ""
		if !showActiveColumns {
			tags = truncate(strings.Join(t.Tags, " "), colTags)
		}

		descStr := t.Description
		if isChild {
			isLast := (i+1 >= len(m.filtered)) || !m.filtered[i+1].IsSubtask()
			prefix := "├─ "
			if isLast {
				prefix = "└─ "
			}
			descStr = prefix + descStr
		}
		desc := truncate(descStr, colDesc)

		var line string
		if showActiveColumns {
			agentStr := truncate(resolveAgent(t, m.agentNames), colAgent)
			stageStr := truncate(resolveStage(t, m.pipelineCfg), colStage)
			line = fmt.Sprintf(" %-*s %-*s %-*s %-*s %-*s %-*s %s",
				colUUID, uuid, colPri, pri,
				colAge, age, colProject, proj,
				colAgent, agentStr, colStage, stageStr, desc)
		} else {
			line = fmt.Sprintf(" %-*s %-*s %-*s %-*s %-*s %s",
				colUUID, uuid, colPri, pri,
				colAge, age, colProject, proj, colTags, tags, desc)
		}

		if selected {
			line = styleSelected.Render(line)
		} else if t.IsToday() && m.filter == filterPending {
			// Today-or-overdue scheduled task: blue background — only in pending view.
			// Uses plain line because lipgloss Width() padding emits ANSI reset sequences
			// that clear the outer background when cells are styled individually.
			line = styleToday.Render(line)
		} else if isChild {
			// Child rows: dim metadata more aggressively
			styledUUID := lipgloss.NewStyle().Width(colUUID).Render(styleDim.Render(uuid))
			styledPri := lipgloss.NewStyle().Width(colPri).Render(styleDim.Render(pri))
			styledAge := lipgloss.NewStyle().Width(colAge).Render(styleDim.Render(age))
			styledProj := lipgloss.NewStyle().Width(colProject).Render(styleDim.Render(proj))
			styledTags := lipgloss.NewStyle().Width(colTags).Render(styleDim.Render(""))
			line = " " + styledUUID + " " + styledPri + " " +
				styledAge + " " + styledProj + " " + styledTags + " " + desc
		} else if showActiveColumns {
			agentStr := truncate(resolveAgent(t, m.agentNames), colAgent)
			styledUUID := lipgloss.NewStyle().Width(colUUID).Render(styleDim.Render(uuid))
			styledPri := lipgloss.NewStyle().Width(colPri).Render(priorityStyle(t.Priority).Render(pri))
			styledAge := lipgloss.NewStyle().Width(colAge).Render(styleDim.Render(age))
			styledProj := lipgloss.NewStyle().Width(colProject).Render(proj)
			styledAgent := lipgloss.NewStyle().Width(colAgent).Render(styleTag.Render(agentStr))
			stageStr := truncate(resolveStage(t, m.pipelineCfg), colStage)
			styledStage := lipgloss.NewStyle().Width(colStage).Render(styleDim.Render(stageStr))
			line = " " + styledUUID + " " + styledPri + " " +
				styledAge + " " + styledProj + " " + styledAgent + " " + styledStage + " " + desc
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

// resolveAgent returns "emoji name" for the agent working on this task.
// Checks tags for agent name match, falls back to spawner UDA.
func resolveAgent(t *Task, agentNames map[string]string) string {
	for _, tag := range t.Tags {
		if emoji, ok := agentNames[tag]; ok {
			if emoji != "" {
				return emoji + " " + tag
			}
			return tag
		}
	}
	if t.Spawner != "" {
		parts := strings.SplitN(t.Spawner, ":", 2)
		if len(parts) == 2 {
			name := parts[1]
			if emoji, ok := agentNames[name]; ok && emoji != "" {
				return emoji + " " + name
			}
			return name
		}
	}
	return ""
}

// resolveStage returns the current pipeline stage display name.
func resolveStage(t *Task, pipeCfg *pipeline.Config) string {
	if pipeCfg == nil {
		return ""
	}
	_, p, err := pipeCfg.MatchPipeline(t.Tags)
	if err != nil || p == nil {
		return ""
	}
	_, stage, err := p.CurrentStage(t.Tags)
	if err != nil || stage == nil {
		return ""
	}
	return stage.DisplayName()
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
