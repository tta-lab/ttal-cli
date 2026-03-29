package tui

import (
	"fmt"
	"strings"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
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

// rowData holds pre-computed display values for a single task row.
type rowData struct {
	t                              *Task
	uuid, pri, age, proj           string
	tags, agentStr, stageStr, desc string
}

func (m Model) buildRowData(i int, c columnLayout) rowData {
	t := &m.filtered[i]
	pri := t.Priority
	if pri == "" {
		pri = "-"
	}
	age := t.Age()
	if age == "" {
		age = "-"
	}
	tags := ""
	if !c.showActive {
		tags = ansi.Truncate(strings.Join(t.Tags, " "), c.tags, "…")
	}
	agentStr, stageStr := "", ""
	if c.showActive {
		agentStr = ansi.Truncate(resolveAgent(t, m.agentEmojiByName), c.agent, "…")
		stageStr = ansi.Truncate(resolveStage(t, m.pipelineCfg), c.stage, "…")
	}
	descStr := t.Description
	if t.IsSubtask() {
		isLast := (i+1 >= len(m.filtered)) || !m.filtered[i+1].IsSubtask()
		prefix := "├─ "
		if isLast {
			prefix = "└─ "
		}
		descStr = prefix + descStr
	}
	return rowData{
		t: t, uuid: t.HexID(), pri: pri, age: age,
		proj: ansi.Truncate(t.Project, c.project, "…"), tags: tags,
		agentStr: agentStr, stageStr: stageStr,
		desc: ansi.Truncate(descStr, c.desc, "…"),
	}
}

func (m Model) renderRow(i int, c columnLayout) string {
	d := m.buildRowData(i, c)
	selected := i == m.cursor
	isChild := d.t.IsSubtask()

	// Plain line (used by selected + today styles)
	plainLine := buildPlainLine(d, c)

	return m.applyRowStyle(d, c, plainLine, selected, isChild)
}

func buildPlainLine(d rowData, c columnLayout) string {
	if c.showActive {
		return fmt.Sprintf(" %-*s %-*s %-*s %-*s %-*s %-*s %s",
			c.uuid, d.uuid, c.pri, d.pri,
			c.age, d.age, c.project, d.proj,
			c.agent, d.agentStr, c.stage, d.stageStr, d.desc)
	}
	return fmt.Sprintf(" %-*s %-*s %-*s %-*s %-*s %s",
		c.uuid, d.uuid, c.pri, d.pri,
		c.age, d.age, c.project, d.proj, c.tags, d.tags, d.desc)
}

func (m Model) applyRowStyle(d rowData, c columnLayout, plainLine string, selected, isChild bool) string {
	switch {
	case selected:
		return styleSelected.Render(plainLine)
	case d.t.IsToday() && m.filter == filterPending:
		// Today-or-overdue scheduled task: blue background — only in pending view.
		// Uses plain line because lipgloss Width() padding emits ANSI reset sequences
		// that clear the outer background when cells are styled individually.
		return styleToday.Render(plainLine)
	case isChild && c.showActive:
		return styleChildRowActive(d, c)
	case isChild:
		return styleChildRow(d, c)
	case c.showActive:
		return styleActiveRow(d, c)
	default:
		return styleStandardRow(d, c)
	}
}

func styleChildRowActive(d rowData, c columnLayout) string {
	sUUID := lipgloss.NewStyle().Width(c.uuid).Render(styleDim.Render(d.uuid))
	sPri := lipgloss.NewStyle().Width(c.pri).Render(styleDim.Render(d.pri))
	sAge := lipgloss.NewStyle().Width(c.age).Render(styleDim.Render(d.age))
	sProj := lipgloss.NewStyle().Width(c.project).Render(styleDim.Render(d.proj))
	sAgent := lipgloss.NewStyle().Width(c.agent).Render(styleDim.Render(""))
	sStage := lipgloss.NewStyle().Width(c.stage).Render(styleDim.Render(""))
	return " " + sUUID + " " + sPri + " " + sAge + " " + sProj + " " + sAgent + " " + sStage + " " + d.desc
}

func styleChildRow(d rowData, c columnLayout) string {
	sUUID := lipgloss.NewStyle().Width(c.uuid).Render(styleDim.Render(d.uuid))
	sPri := lipgloss.NewStyle().Width(c.pri).Render(styleDim.Render(d.pri))
	sAge := lipgloss.NewStyle().Width(c.age).Render(styleDim.Render(d.age))
	sProj := lipgloss.NewStyle().Width(c.project).Render(styleDim.Render(d.proj))
	sTags := lipgloss.NewStyle().Width(c.tags).Render(styleDim.Render(""))
	return " " + sUUID + " " + sPri + " " + sAge + " " + sProj + " " + sTags + " " + d.desc
}

func styleActiveRow(d rowData, c columnLayout) string {
	sUUID := lipgloss.NewStyle().Width(c.uuid).Render(styleDim.Render(d.uuid))
	sPri := lipgloss.NewStyle().Width(c.pri).Render(priorityStyle(d.t.Priority).Render(d.pri))
	sAge := lipgloss.NewStyle().Width(c.age).Render(styleDim.Render(d.age))
	sProj := lipgloss.NewStyle().Width(c.project).Render(d.proj)
	sAgent := lipgloss.NewStyle().Width(c.agent).Render(styleTag.Render(d.agentStr))
	sStage := lipgloss.NewStyle().Width(c.stage).Render(styleDim.Render(d.stageStr))
	return " " + sUUID + " " + sPri + " " + sAge + " " + sProj + " " + sAgent + " " + sStage + " " + d.desc
}

func styleStandardRow(d rowData, c columnLayout) string {
	sUUID := lipgloss.NewStyle().Width(c.uuid).Render(styleDim.Render(d.uuid))
	sPri := lipgloss.NewStyle().Width(c.pri).Render(priorityStyle(d.t.Priority).Render(d.pri))
	sAge := lipgloss.NewStyle().Width(c.age).Render(styleDim.Render(d.age))
	sProj := lipgloss.NewStyle().Width(c.project).Render(d.proj)
	sTags := lipgloss.NewStyle().Width(c.tags).Render(styleTag.Render(d.tags))
	return " " + sUUID + " " + sPri + " " + sAge + " " + sProj + " " + sTags + " " + d.desc
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
