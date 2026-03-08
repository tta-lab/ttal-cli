package tui

import (
	"fmt"
	"strings"

	"charm.land/bubbles/v2/textinput"
	"charm.land/lipgloss/v2"
)

func (m Model) viewTextInputOverlay(background, title, prompt string, input textinput.Model) string {
	var b strings.Builder

	b.WriteString(styleTitle.Render(title))
	b.WriteString("\n\n")
	b.WriteString("  " + styleDim.Render(prompt) + "\n")
	b.WriteString("  ")
	b.WriteString(input.View())
	b.WriteString("\n\n")
	b.WriteString(styleDim.Render("  Enter:confirm  Esc:cancel"))

	overlay := styleOverlay.
		Width(50).
		Render(b.String())

	return m.placeOverlay(background, overlay)
}

func (m Model) viewRouteOverlay(background string) string {
	var b strings.Builder

	b.WriteString(styleTitle.Render("Route to Agent"))
	b.WriteString("\n\n")
	b.WriteString("  ")
	b.WriteString(m.routeInput.View())
	b.WriteString("\n\n")

	if len(m.routeMatches) == 0 {
		b.WriteString(styleDim.Render("  No matching agents"))
	} else {
		for i, a := range m.routeMatches {
			prefix := "  "
			if i == 0 {
				prefix = "> "
			}
			emoji := a.Emoji
			if emoji == "" {
				emoji = " "
			}
			var line string
			if a.Role != "" {
				role := styleDim.Render("(" + a.Role + ")")
				line = fmt.Sprintf("%s%s %s %s", prefix, emoji, a.Name, role)
			} else {
				line = fmt.Sprintf("%s%s %s %s", prefix, emoji, a.Name, styleDim.Render("(no role)"))
			}
			if i == 0 {
				line = styleSelected.Render(line)
			} else if a.Role == "" {
				line = styleDim.Render(line)
			}
			b.WriteString(line + "\n")
			if i >= 9 {
				b.WriteString(styleDim.Render(fmt.Sprintf("  ... and %d more", len(m.routeMatches)-10)))
				break
			}
		}
	}

	b.WriteString("\n")
	b.WriteString(styleDim.Render("  Enter:select  Tab:complete  Esc:cancel"))

	overlay := styleOverlay.
		Width(50).
		Render(b.String())

	return m.placeOverlay(background, overlay)
}

func (m Model) viewSearchOverlay(background string) string {
	return m.viewModifyMatchesOverlay(
		background, "Search Tasks",
		"Filter (e.g. project:x +tag priority:H):",
		m.searchInput, "Enter:search",
	)
}

func (m Model) viewConfirmDeleteOverlay(background string) string {
	t := m.selectedTask()
	desc := "(no task selected)"
	if t != nil {
		desc = truncate(t.Description, 40)
	}

	var b strings.Builder
	b.WriteString(lipgloss.NewStyle().Bold(true).Foreground(colorRed).Render("Delete Task"))
	b.WriteString("\n\n")
	b.WriteString(styleDim.Render("  " + desc + "\n\n"))
	b.WriteString("  Delete task? [y/N]\n\n")
	b.WriteString(styleDim.Render("  y:confirm  any other key:cancel"))

	overlay := styleOverlay.
		Width(50).
		Render(b.String())

	return m.placeOverlay(background, overlay)
}

func (m Model) placeOverlay(background, overlay string) string {
	const overlayWidth = 54
	bgLines := strings.Split(background, "\n")
	overlayLines := strings.Split(overlay, "\n")

	startRow := (m.height - len(overlayLines)) / 2
	if startRow < 0 {
		startRow = 0
	}
	startCol := (m.width - overlayWidth) / 2
	if startCol < 0 {
		startCol = 0
	}

	for len(bgLines) < m.height {
		bgLines = append(bgLines, "")
	}

	for i, overlayLine := range overlayLines {
		row := startRow + i
		if row >= len(bgLines) {
			break
		}
		bg := bgLines[row]
		for len(bg) < startCol {
			bg += " "
		}
		bgLines[row] = bg[:min(startCol, len(bg))] + overlayLine
	}

	return strings.Join(bgLines, "\n")
}

func (m Model) viewModifyMatchesOverlay(
	background, title, prompt string, input textinput.Model, helpText string,
) string {
	var b strings.Builder

	b.WriteString(styleTitle.Render(title))
	b.WriteString("\n\n")
	b.WriteString(styleDim.Render("  " + prompt + "\n"))
	b.WriteString("  ")
	b.WriteString(input.View())
	b.WriteString("\n\n")

	if len(m.modifyMatches) == 0 {
		b.WriteString(styleDim.Render("  No matching suggestions"))
	} else {
		for i, match := range m.modifyMatches {
			prefix := "  "
			if i == m.modifyIndex {
				prefix = "> "
			}
			var value string
			switch match.Type {
			case matchTypeProject:
				value = match.Value
			case matchTypeTag:
				value = match.Value
			default:
				value = match.Value
			}
			line := fmt.Sprintf("%s[%s] %s", prefix, match.Type, value)
			if i == m.modifyIndex {
				line = styleSelected.Render(line)
			} else {
				line = styleDim.Render(line)
			}
			b.WriteString(line + "\n")
			if i >= 9 {
				b.WriteString(styleDim.Render(fmt.Sprintf("  ... and %d more", len(m.modifyMatches)-10)))
				break
			}
		}
	}

	b.WriteString("\n")
	b.WriteString(styleDim.Render("  " + helpText + "  Tab/Ctrl+N:next  Ctrl+P:prev  Ctrl+W:word  Esc:cancel"))

	overlay := styleOverlay.
		Width(50).
		Render(b.String())

	return m.placeOverlay(background, overlay)
}

func (m Model) viewModifyOverlay(background string) string {
	return m.viewModifyMatchesOverlay(
		background, "Modify Task",
		"Modifiers (e.g. project:x +tag priority:H):",
		m.modifyInput, "Enter:confirm",
	)
}
