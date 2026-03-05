package tui

import (
	"fmt"
	"strings"
)

func (m Model) viewRouteOverlay(background string) string {
	var b strings.Builder

	b.WriteString(styleTitle.Render("Route to Agent"))
	b.WriteString("\n\n")
	b.WriteString(fmt.Sprintf("  > %s_\n\n", m.routeInput))

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
			role := styleDim.Render("(" + a.Role + ")")
			line := fmt.Sprintf("%s%s %s %s", prefix, emoji, a.Name, role)
			if i == 0 {
				line = styleSelected.Render(line)
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
		Width(40).
		Render(b.String())

	// Center the overlay on the background
	bgLines := strings.Split(background, "\n")
	overlayLines := strings.Split(overlay, "\n")

	startRow := (m.height - len(overlayLines)) / 2
	if startRow < 0 {
		startRow = 0
	}
	startCol := (m.width - 44) / 2 // 40 + padding
	if startCol < 0 {
		startCol = 0
	}

	// Place overlay on top of background
	for len(bgLines) < m.height {
		bgLines = append(bgLines, "")
	}

	for i, overlayLine := range overlayLines {
		row := startRow + i
		if row >= len(bgLines) {
			break
		}
		bg := bgLines[row]
		// Pad background line to overlay start
		for len(bg) < startCol {
			bg += " "
		}
		bgLines[row] = bg[:min(startCol, len(bg))] + overlayLine
	}

	return strings.Join(bgLines, "\n")
}
