package tui

import (
	"fmt"
	"strings"

	"charm.land/lipgloss/v2"
	"github.com/tta-lab/ttal-cli/internal/enrichment"
	"github.com/tta-lab/ttal-cli/internal/project"
	"github.com/tta-lab/ttal-cli/internal/taskwarrior"
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
	writeAnnotations(&b, t, m.width)

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
	displayBranch := t.Branch
	if displayBranch == "" {
		displayBranch = enrichment.GenerateBranch(t.Description)
	}
	if displayBranch != "" {
		field(b, "Branch:", " ", displayBranch)
	}
	if path := project.ResolveProjectPath(t.Project); path != "" {
		field(b, "Path:", "   ", path)
	}
	if t.PRID != "" {
		info, err := taskwarrior.ParsePRID(t.PRID)
		if err == nil && info.LGTM {
			field(b, "PR:", "    ", fmt.Sprintf("#%d ✓", info.Index))
		} else {
			field(b, "PR:", "    ", "#"+t.PRID)
		}
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

func writeAnnotations(b *strings.Builder, t *Task, width int) {
	if len(t.Annotations) == 0 {
		return
	}
	b.WriteString("\n  " + styleTitle.Render("Annotations") + "\n")

	// datePrefix: "  " (2) + "2006-01-02" (10) + " " (1) = 13; no-date path is just "  " (2)
	const datePrefixLen = 13

	for _, ann := range t.Annotations {
		date := ""
		effPrefix := 2 // "  " only
		if ann.Entry != "" {
			date = styleDim.Render(formatDate(ann.Entry) + " ")
			effPrefix = datePrefixLen
		}

		desc := ann.Description
		if width > effPrefix+1 {
			desc = lipgloss.Wrap(ann.Description, width-effPrefix, " ")
			// Indent continuation lines to align under first line of text
			indent := strings.Repeat(" ", effPrefix)
			parts := strings.SplitAfter(desc, "\n")
			for i := 1; i < len(parts); i++ {
				if parts[i] != "" {
					parts[i] = indent + parts[i]
				}
			}
			desc = strings.Join(parts, "")
		}

		fmt.Fprintf(b, "  %s%s\n", date, desc)
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
	teamLabel := m.teamName
	if teamLabel == "" {
		teamLabel = "default"
	}
	b.WriteString(styleTitle.Render(fmt.Sprintf(" TTal [%s] — Help", teamLabel)))
	b.WriteString("\n\n")
	b.WriteString(helpText)
	b.WriteString("\n\n")
	b.WriteString(styleDim.Render("  Press ? or Esc to close"))
	return m.padToHeight(b.String()) + m.viewStatusBar()
}
