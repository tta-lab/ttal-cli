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
	fmt.Fprintf(os.Stderr, "\n%s\n", askCmdStyle.Render("  $ "+command))
}

// renderCommandResult prints truncated command output and exit code to stderr.
func renderCommandResult(output string, exitCode int) {
	if output != "" {
		truncated := truncateOutput(output)
		fmt.Fprintf(os.Stderr, "%s\n", askOutputStyle.Render(truncated))
	}
	if exitCode != 0 {
		fmt.Fprintf(os.Stderr, "%s\n", askExitErrStyle.Render(
			fmt.Sprintf("  exit %d", exitCode)))
	}
}

// renderRetry prints a styled retry notice to stderr.
// It serves as the OnRetry callback for logos.Callbacks.
func renderRetry(reason string, step int) {
	fmt.Fprintf(os.Stderr, "\n%s\n", askRetryStyle.Render(
		fmt.Sprintf("  ↺ retry (step %d: %s)", step, reason)))
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
