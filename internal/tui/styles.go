package tui

import "charm.land/lipgloss/v2"

var (
	colorDim     = lipgloss.Color("241")
	colorAccent  = lipgloss.Color("99")
	colorRed     = lipgloss.Color("196")
	colorYellow  = lipgloss.Color("220")
	colorGreen   = lipgloss.Color("78")
	colorCyan    = lipgloss.Color("86")
	colorSubtle  = lipgloss.Color("245")
	colorOverlay = lipgloss.Color("236")

	styleTitle = lipgloss.NewStyle().
			Bold(true).
			Foreground(colorAccent)

	styleStatusBar = lipgloss.NewStyle().
			Foreground(colorSubtle).
			Padding(0, 1)

	styleSelected = lipgloss.NewStyle().
			Bold(true).
			Foreground(colorCyan)

	styleDim = lipgloss.NewStyle().
			Foreground(colorDim)

	styleTag = lipgloss.NewStyle().
			Foreground(colorAccent)

	stylePriorityH = lipgloss.NewStyle().Foreground(colorRed).Bold(true)
	stylePriorityM = lipgloss.NewStyle().Foreground(colorYellow)
	stylePriorityL = lipgloss.NewStyle().Foreground(colorGreen)

	styleOverlay = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(colorAccent).
			Padding(1, 2)

	styleHelp = lipgloss.NewStyle().
			Foreground(colorDim)
)

func priorityStyle(p string) lipgloss.Style {
	switch p {
	case "H":
		return stylePriorityH
	case "M":
		return stylePriorityM
	case "L":
		return stylePriorityL
	default:
		return styleDim
	}
}
