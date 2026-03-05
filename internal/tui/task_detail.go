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

	// Core fields
	field(&b, "UUID:", "  ", t.UUID)
	field(&b, "ID:", "    ", fmt.Sprintf("%d", t.ID))
	field(&b, "Desc:", "  ", t.Description)
	field(&b, "Status:", " ", t.Status)

	writeOptionalFields(&b, t)
	writeAnnotations(&b, t)

	b.WriteString("\n")
	b.WriteString(styleDim.Render(
		"  x:execute  r:route  d:done  m:modify  A:annotate  o:PR  s:session  a:today  Esc:back"))

	return m.padToHeight(b.String()) + m.viewStatusBar()
}

func writeOptionalFields(b *strings.Builder, t *Task) {
	if t.Project != "" {
		field(b, "Proj:", "   ", t.Project)
	}
	if t.Priority != "" {
		field(b, "Prior:", "  ", priorityStyle(t.Priority).Render(t.Priority))
	}
	if len(t.Tags) > 0 {
		field(b, "Tags:", "  ", styleTag.Render(strings.Join(t.Tags, ", ")))
	}
	if t.Urgency != 0 {
		field(b, "Urg:", "   ", fmt.Sprintf("%.1f", t.Urgency))
	}
	if t.Branch != "" {
		field(b, "Branch:", " ", t.Branch)
	}
	if t.ProjectPath != "" {
		field(b, "Path:", "   ", t.ProjectPath)
	}
	if t.PRID != "" {
		field(b, "PR:", "    ", "#"+t.PRID)
	}
	if t.Spawner != "" {
		field(b, "Spawner:", " ", t.Spawner)
	}
	if t.Scheduled != "" {
		field(b, "Sched:", " ", formatDate(t.Scheduled))
	}
	if t.Due != "" {
		field(b, "Due:", "   ", formatDate(t.Due))
	}
	if t.Start != "" {
		field(b, "Started:", " ", formatDate(t.Start))
	}
}

func writeAnnotations(b *strings.Builder, t *Task) {
	if len(t.Annotations) == 0 {
		return
	}
	b.WriteString("\n  " + styleTitle.Render("Annotations") + "\n")
	for _, ann := range t.Annotations {
		date := ""
		if ann.Entry != "" {
			date = styleDim.Render(formatDate(ann.Entry) + " ")
		}
		fmt.Fprintf(b, "  %s%s\n", date, ann.Description)
	}
}

func field(b *strings.Builder, label, pad, value string) {
	fmt.Fprintf(b, "  %s%s%s\n", styleDim.Render(label), pad, value)
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
