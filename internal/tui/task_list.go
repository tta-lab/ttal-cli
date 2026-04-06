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
	cell := func(title string, width int) string {
		return lipgloss.NewStyle().Width(width).MaxWidth(width).Inline(true).Render(title)
	}

	var parts []string
	parts = append(parts, cell("ID", c.uuid))
	parts = append(parts, cell("P", c.pri))
	parts = append(parts, cell("Age", c.age))
	parts = append(parts, cell("Project", c.project))
	if c.showActive {
		parts = append(parts, cell("Agent", c.agent))
		parts = append(parts, cell("Stage", c.stage))
	} else {
		parts = append(parts, cell("Tags", c.tags))
	}
	parts = append(parts, "Description")

	return styleDim.Render(" " + strings.Join(parts, " "))
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
		tags = strings.Join(t.Tags, " ")
	}
	agentStr, stageStr := "", ""
	if c.showActive {
		agentStr = resolveAgent(t, m.agentEmojiByName)
		stageStr = resolveStage(t, m.pipelineCfg)
	}
	return rowData{
		t: t, uuid: t.HexID(), pri: pri, age: age,
		proj: t.Project, tags: tags,
		agentStr: agentStr, stageStr: stageStr,
		desc: t.Description,
	}
}

func (m Model) renderRow(i int, c columnLayout) string {
	d := m.buildRowData(i, c)
	selected := i == m.cursor

	cellLine := renderCells(d, c)

	return m.applyRowStyle(d, c, cellLine, selected)
}

// renderCells builds a row string with lipgloss-width-aware cell alignment.
// Values are plain text — caller applies row-level styling (selected, today) afterward.
// Uses Inline(true) to avoid ANSI reset sequences that would interfere with
// outer row-level styling (e.g., styleToday's background color).
func renderCells(d rowData, c columnLayout) string {
	cell := func(value string, width int) string {
		return lipgloss.NewStyle().Width(width).MaxWidth(width).Inline(true).
			Render(ansi.Truncate(value, width, "…"))
	}

	var parts []string
	parts = append(parts, cell(d.uuid, c.uuid))
	parts = append(parts, cell(d.pri, c.pri))
	parts = append(parts, cell(d.age, c.age))
	parts = append(parts, cell(d.proj, c.project))
	if c.showActive {
		parts = append(parts, cell(d.agentStr, c.agent))
		parts = append(parts, cell(d.stageStr, c.stage))
	} else {
		parts = append(parts, cell(d.tags, c.tags))
	}
	parts = append(parts, ansi.Truncate(d.desc, c.desc, "…"))

	return " " + strings.Join(parts, " ")
}

func (m Model) applyRowStyle(d rowData, c columnLayout, plainLine string, selected bool) string {
	switch {
	case selected:
		return styleSelected.Render(plainLine)
	case d.t.IsToday() && m.filter == filterPending:
		// Today-or-overdue scheduled task: blue background — only in pending view.
		// Uses plain line because lipgloss Width() padding emits ANSI reset sequences
		// that clear the outer background when cells are styled individually.
		return styleToday.Render(plainLine)
	case c.showActive:
		return styleActiveRow(d, c)
	default:
		return styleStandardRow(d, c)
	}
}

// styledCell renders a value with inner styling inside a fixed-width cell.
func styledCell(value string, width int, style lipgloss.Style) string {
	return lipgloss.NewStyle().Width(width).MaxWidth(width).Inline(true).
		Render(style.Render(ansi.Truncate(value, width, "…")))
}

func styleActiveRow(d rowData, c columnLayout) string {
	parts := []string{
		styledCell(d.uuid, c.uuid, styleDim),
		styledCell(d.pri, c.pri, priorityStyle(d.t.Priority)),
		styledCell(d.age, c.age, styleDim),
		styledCell(d.proj, c.project, lipgloss.NewStyle()),
		styledCell(d.agentStr, c.agent, styleTag),
		styledCell(d.stageStr, c.stage, styleDim),
		ansi.Truncate(d.desc, c.desc, "…"),
	}
	return " " + strings.Join(parts, " ")
}

func styleStandardRow(d rowData, c columnLayout) string {
	parts := []string{
		styledCell(d.uuid, c.uuid, styleDim),
		styledCell(d.pri, c.pri, priorityStyle(d.t.Priority)),
		styledCell(d.age, c.age, styleDim),
		styledCell(d.proj, c.project, lipgloss.NewStyle()),
		styledCell(d.tags, c.tags, styleTag),
		ansi.Truncate(d.desc, c.desc, "…"),
	}
	return " " + strings.Join(parts, " ")
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

	left := styleStatusBar.Render(strings.Join(parts, "  "))
	leftWidth := lipgloss.Width(left)

	// Context-sensitive help on the right
	m.helpModel.SetWidth(m.width - leftWidth - 2)
	right := m.helpModel.View(m.keys)

	gap := m.width - leftWidth - lipgloss.Width(right)
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
