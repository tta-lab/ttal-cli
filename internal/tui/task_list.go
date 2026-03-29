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

	cols := m.buildColumns()
	b.WriteString(m.renderHeader(cols))
	b.WriteString("\n")

	visible := m.visibleRows()
	end := m.offset + visible
	if end > len(m.filtered) {
		end = len(m.filtered)
	}

	for i := m.offset; i < end; i++ {
		b.WriteString(m.renderRow(i, cols))
		b.WriteString("\n")
	}

	result := m.padToHeight(b.String())
	result += m.viewStatusBar()
	return result
}

// columnLayout holds pre-computed column widths and mode flags.
type columnLayout struct {
	uuid, pri, age, project, tags, agent, stage, desc int
	showActive                                        bool
}

func (m Model) buildColumns() columnLayout {
	c := columnLayout{
		uuid:    9,
		pri:     2,
		age:     5,
		project: 10,
		tags:    10,
	}
	c.showActive = m.filter == filterActive
	if c.showActive {
		c.agent = 10
		c.stage = 12
		c.tags = 0
	}

	overhead := c.uuid + c.pri + c.age + c.project + 7
	if c.showActive {
		overhead += c.agent + c.stage + 2
	} else {
		overhead += c.tags
	}
	c.desc = m.width - overhead
	if c.desc < 20 {
		c.desc = 20
	}
	return c
}

func (m Model) renderHeader(c columnLayout) string {
	if c.showActive {
		return styleDim.Render(
			fmt.Sprintf(" %-*s %-*s %-*s %-*s %-*s %-*s %s",
				c.uuid, "ID", c.pri, "P",
				c.age, "Age", c.project, "Project",
				c.agent, "Agent", c.stage, "Stage", "Description"))
	}
	return styleDim.Render(
		fmt.Sprintf(" %-*s %-*s %-*s %-*s %-*s %s",
			c.uuid, "ID", c.pri, "P",
			c.age, "Age", c.project, "Project", c.tags, "Tags", "Description"))
}

func (m Model) renderRow(i int, c columnLayout) string {
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
	proj := truncate(t.Project, c.project)

	tags := ""
	if !c.showActive {
		tags = truncate(strings.Join(t.Tags, " "), c.tags)
	}

	// Agent/stage resolved once, used in both plain and styled paths
	agentStr := ""
	stageStr := ""
	if c.showActive {
		agentStr = truncate(resolveAgent(t, m.agentEmojiByName), c.agent)
		stageStr = truncate(resolveStage(t, m.pipelineCfg), c.stage)
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
	desc := truncate(descStr, c.desc)

	// Plain line (used by selected + today styles)
	var plainLine string
	if c.showActive {
		plainLine = fmt.Sprintf(" %-*s %-*s %-*s %-*s %-*s %-*s %s",
			c.uuid, uuid, c.pri, pri,
			c.age, age, c.project, proj,
			c.agent, agentStr, c.stage, stageStr, desc)
	} else {
		plainLine = fmt.Sprintf(" %-*s %-*s %-*s %-*s %-*s %s",
			c.uuid, uuid, c.pri, pri,
			c.age, age, c.project, proj, c.tags, tags, desc)
	}

	switch {
	case selected:
		return styleSelected.Render(plainLine)
	case t.IsToday() && m.filter == filterPending:
		// Today-or-overdue scheduled task: blue background — only in pending view.
		// Uses plain line because lipgloss Width() padding emits ANSI reset sequences
		// that clear the outer background when cells are styled individually.
		return styleToday.Render(plainLine)
	case isChild && c.showActive:
		// Child rows in active view: use active column layout with dim styling
		styledUUID := lipgloss.NewStyle().Width(c.uuid).Render(styleDim.Render(uuid))
		styledPri := lipgloss.NewStyle().Width(c.pri).Render(styleDim.Render(pri))
		styledAge := lipgloss.NewStyle().Width(c.age).Render(styleDim.Render(age))
		styledProj := lipgloss.NewStyle().Width(c.project).Render(styleDim.Render(proj))
		styledAgent := lipgloss.NewStyle().Width(c.agent).Render(styleDim.Render(""))
		styledStage := lipgloss.NewStyle().Width(c.stage).Render(styleDim.Render(""))
		return " " + styledUUID + " " + styledPri + " " +
			styledAge + " " + styledProj + " " + styledAgent + " " + styledStage + " " + desc
	case isChild:
		// Child rows in standard views: dim all metadata
		styledUUID := lipgloss.NewStyle().Width(c.uuid).Render(styleDim.Render(uuid))
		styledPri := lipgloss.NewStyle().Width(c.pri).Render(styleDim.Render(pri))
		styledAge := lipgloss.NewStyle().Width(c.age).Render(styleDim.Render(age))
		styledProj := lipgloss.NewStyle().Width(c.project).Render(styleDim.Render(proj))
		styledTags := lipgloss.NewStyle().Width(c.tags).Render(styleDim.Render(""))
		return " " + styledUUID + " " + styledPri + " " +
			styledAge + " " + styledProj + " " + styledTags + " " + desc
	case c.showActive:
		styledUUID := lipgloss.NewStyle().Width(c.uuid).Render(styleDim.Render(uuid))
		styledPri := lipgloss.NewStyle().Width(c.pri).Render(priorityStyle(t.Priority).Render(pri))
		styledAge := lipgloss.NewStyle().Width(c.age).Render(styleDim.Render(age))
		styledProj := lipgloss.NewStyle().Width(c.project).Render(proj)
		styledAgent := lipgloss.NewStyle().Width(c.agent).Render(styleTag.Render(agentStr))
		styledStage := lipgloss.NewStyle().Width(c.stage).Render(styleDim.Render(stageStr))
		return " " + styledUUID + " " + styledPri + " " +
			styledAge + " " + styledProj + " " + styledAgent + " " + styledStage + " " + desc
	default:
		styledUUID := lipgloss.NewStyle().Width(c.uuid).Render(styleDim.Render(uuid))
		styledPri := lipgloss.NewStyle().Width(c.pri).Render(priorityStyle(t.Priority).Render(pri))
		styledAge := lipgloss.NewStyle().Width(c.age).Render(styleDim.Render(age))
		styledProj := lipgloss.NewStyle().Width(c.project).Render(proj)
		styledTags := lipgloss.NewStyle().Width(c.tags).Render(styleTag.Render(tags))
		return " " + styledUUID + " " + styledPri + " " +
			styledAge + " " + styledProj + " " + styledTags + " " + desc
	}
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
func resolveAgent(t *Task, agentEmojiByName map[string]string) string {
	for _, tag := range t.Tags {
		if emoji, ok := agentEmojiByName[tag]; ok {
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
			if emoji, ok := agentEmojiByName[name]; ok && emoji != "" {
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
