package format

import (
	"image/color"

	"charm.land/lipgloss/v2"
)

// TableStyles returns the shared lipgloss styles used across all ttal list tables.
// Returns: dimColor, headerStyle, cellStyle, dimStyle
func TableStyles() (color.Color, lipgloss.Style, lipgloss.Style, lipgloss.Style) {
	dim := lipgloss.Color("241")
	header := lipgloss.NewStyle().Bold(true).Padding(0, 1)
	cell := lipgloss.NewStyle().Padding(0, 1)
	return dim, header, cell, cell.Foreground(dim)
}
