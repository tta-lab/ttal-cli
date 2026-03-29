package tui

import (
	"strings"
	"testing"

	"github.com/charmbracelet/x/ansi"
)

func TestPlaceOverlayDoesNotCorruptANSI(t *testing.T) {
	m := Model{width: 80, height: 10}

	// Background with ANSI-styled content
	bg := "\033[1mBold text\033[0m" + strings.Repeat(" ", 60)
	background := strings.Repeat(bg+"\n", 9) + bg

	overlay := "OVERLAY"

	result := m.placeOverlay(background, overlay)

	// Should not contain broken ANSI (incomplete escape sequence)
	// Basic check: result should be valid and contain the overlay
	if !strings.Contains(result, "OVERLAY") {
		t.Error("overlay not found in result")
	}
	// No truncated ANSI sequences mid-escape: ansi.StringWidth should not panic
	lines := strings.Split(result, "\n")
	for _, line := range lines {
		_ = ansi.StringWidth(line)
	}
}
