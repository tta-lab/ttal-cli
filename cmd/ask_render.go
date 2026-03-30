package cmd

import (
	"fmt"
	"os"
	"strings"

	"charm.land/lipgloss/v2"
)

var (
	askCmdStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("241")).
			Bold(true)

	askOutputStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("245")).
			PaddingLeft(2)

	askExitErrStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("196")).
			Bold(true)

	askRetryStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("214")).
			Bold(true)
)

// renderCommandStart prints a styled command header to stderr.
func renderCommandStart(command string) {
	lipgloss.Fprintf(os.Stderr, "\n%s\n", askCmdStyle.Render("  $ "+command))
}

// renderCommandResult prints truncated command output and exit code to stderr.
func renderCommandResult(output string, exitCode int) {
	if output != "" {
		truncated := truncateOutput(output)
		lipgloss.Fprintf(os.Stderr, "%s\n", askOutputStyle.Render(truncated))
	}
	if exitCode != 0 {
		lipgloss.Fprintf(os.Stderr, "%s\n", askExitErrStyle.Render(
			fmt.Sprintf("  exit %d", exitCode)))
	}
}

// renderRetry prints a styled retry notice to stderr.
// It serves as the OnRetry callback for logos.Callbacks.
func renderRetry(reason string, step int) {
	lipgloss.Fprintf(os.Stderr, "\n%s\n", askRetryStyle.Render(
		fmt.Sprintf("  ↺ retry (step %d: %s)", step, reason)))
}

// renderDelta handles OnDelta text — prose goes to stdout, <cmd> blocks
// are rendered as styled command previews on stderr.
func renderDelta(text string) {
	for len(text) > 0 {
		start := strings.Index(text, "<cmd>")
		if start == -1 {
			fmt.Print(text)
			return
		}
		// Print prose before the block
		if start > 0 {
			fmt.Print(text[:start])
		}
		end := strings.Index(text[start:], "</cmd>")
		if end == -1 {
			// Partial block — shouldn't happen with atomic chunks, print as-is
			fmt.Print(text[start:])
			return
		}
		blockContent := text[start+len("<cmd>") : start+end]
		renderCmdBlock(blockContent)
		text = text[start+end+len("</cmd>"):]
	}
}

// renderCmdBlock renders the contents of a <cmd> block as styled command
// lines on stderr. This shows the user what the agent intends to run
// as soon as it streams, before execution begins.
func renderCmdBlock(content string) {
	lines := strings.Split(strings.TrimSpace(content), "\n")
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			continue
		}
		// Strip the § prefix for display, show as "$ command"
		trimmed = strings.TrimPrefix(trimmed, "§ ")
		renderCommandStart(trimmed)
	}
}

const askMaxOutputLines = 10

// truncateOutput limits output to askMaxOutputLines, appending a count if truncated.
func truncateOutput(output string) string {
	lines := strings.Split(strings.TrimRight(output, "\n"), "\n")
	if len(lines) <= askMaxOutputLines {
		return strings.Join(lines, "\n")
	}
	suffix := fmt.Sprintf("... (%d more lines)", len(lines)-askMaxOutputLines)
	truncated := append(lines[:askMaxOutputLines:askMaxOutputLines], suffix)
	return strings.Join(truncated, "\n")
}
