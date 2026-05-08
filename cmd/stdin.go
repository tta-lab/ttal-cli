package cmd

import (
	"fmt"
	"io"
	"os"
	"strings"

	"golang.org/x/term"
)

// readStdinIfPiped reads stdin only when the current process is not attached to
// an interactive terminal.
//
// Boundary: ttal send is a stdin-driven CLI for controlled agent runtimes
// (bash tool calls / sandbox command execution). It is not the generic entry
// point for arbitrary background processes. Process-style integrations should
// talk to the ttal daemon directly instead of shelling out to this command.
//
// Within that boundary, term.IsTerminal is sufficient: a tty means "no piped
// input", while non-tty stdin is expected to be a real pipe / heredoc payload
// supplied by the runtime.
func readStdinIfPiped() (string, error) {
	if term.IsTerminal(int(os.Stdin.Fd())) {
		return "", nil
	}
	data, err := io.ReadAll(os.Stdin)
	if err != nil {
		return "", fmt.Errorf("read stdin: %w", err)
	}
	return strings.TrimRight(string(data), "\n"), nil
}
