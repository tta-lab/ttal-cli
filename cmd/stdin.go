package cmd

import (
	"fmt"
	"io"
	"os"
	"strings"

	"golang.org/x/term"
)

// readStdinIfPiped reads from stdin if it is a pipe, otherwise returns an empty string.
// This prevents io.ReadAll from blocking on a tty waiting for Ctrl-D.
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
