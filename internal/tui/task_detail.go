package tui

import (
	"fmt"
	"strings"
)

func (m Model) viewTaskDetail() string {
	t := m.selectedTask()
	if t == nil {
		return "No task selected"
	}

	var b strings.Builder

	b.WriteString(styleTitle.Render(" Task Detail"))
	b.WriteString("\n\n")

	// Metadata
	b.WriteString(fmt.Sprintf("  %s  %s\n", styleDim.Render("UUID:"), t.UUID))
	b.WriteString(fmt.Sprintf("  %s    %s\n", styleDim.Render("ID:"), fmt.Sprintf("%d", t.ID)))
	b.WriteString(fmt.Sprintf("  %s  %s\n", styleDim.Render("Desc:"), t.Description))
	b.WriteString(fmt.Sprintf("  %s %s\n", styleDim.Render("Status:"), t.Status))

	if t.Project != "" {
		b.WriteString(fmt.Sprintf("  %s   %s\n", styleDim.Render("Proj:"), t.Project))
	}
	if t.Priority != "" {
		b.WriteString(fmt.Sprintf("  %s  %s\n", styleDim.Render("Prior:"), priorityStyle(t.Priority).Render(t.Priority)))
	}
	if len(t.Tags) > 0 {
		b.WriteString(fmt.Sprintf("  %s  %s\n", styleDim.Render("Tags:"), styleTag.Render(strings.Join(t.Tags, ", "))))
	}
	if t.Urgency != 0 {
		b.WriteString(fmt.Sprintf("  %s   %s\n", styleDim.Render("Urg:"), fmt.Sprintf("%.1f", t.Urgency)))
	}

	// Worker UDAs
	if t.Branch != "" {
		b.WriteString(fmt.Sprintf("  %s %s\n", styleDim.Render("Branch:"), t.Branch))
	}
	if t.ProjectPath != "" {
		b.WriteString(fmt.Sprintf("  %s   %s\n", styleDim.Render("Path:"), t.ProjectPath))
	}
	if t.PRID != "" {
		b.WriteString(fmt.Sprintf("  %s    %s\n", styleDim.Render("PR:"), "#"+t.PRID))
	}
	if t.Spawner != "" {
		b.WriteString(fmt.Sprintf("  %s %s\n", styleDim.Render("Spawner:"), t.Spawner))
	}

	// Dates
	if t.Scheduled != "" {
		b.WriteString(fmt.Sprintf("  %s %s\n", styleDim.Render("Sched:"), formatDate(t.Scheduled)))
	}
	if t.Due != "" {
		b.WriteString(fmt.Sprintf("  %s   %s\n", styleDim.Render("Due:"), formatDate(t.Due)))
	}
	if t.Start != "" {
		b.WriteString(fmt.Sprintf("  %s %s\n", styleDim.Render("Started:"), formatDate(t.Start)))
	}

	// Annotations
	if len(t.Annotations) > 0 {
		b.WriteString("\n  " + styleTitle.Render("Annotations") + "\n")
		for _, ann := range t.Annotations {
			date := ""
			if ann.Entry != "" {
				date = styleDim.Render(formatDate(ann.Entry) + " ")
			}
			b.WriteString(fmt.Sprintf("  %s%s\n", date, ann.Description))
		}
	}

	// Available actions
	b.WriteString("\n")
	actions := styleDim.Render("  x:execute  r:route  o:PR  s:session  t:term  e:editor  a:today  Esc:back")
	b.WriteString(actions)

	return m.padToHeight(b.String()) + m.viewStatusBar()
}

func formatDate(s string) string {
	if t, err := parseTaskDate(s); err == nil {
		return t.Format("2006-01-02")
	}
	return s
}

func (m Model) viewHelp() string {
	var b strings.Builder
	b.WriteString(styleTitle.Render(" ttal tui — Help"))
	b.WriteString("\n\n")
	b.WriteString(helpText)
	b.WriteString("\n\n")
	b.WriteString(styleDim.Render("  Press ? or Esc to close"))
	return m.padToHeight(b.String()) + m.viewStatusBar()
}
