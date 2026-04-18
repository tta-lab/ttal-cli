package cmd

import (
	"fmt"
	"io"
	"os"
	"strings"
)

// readStdinIfPiped reads from stdin if it is a pipe, otherwise returns an empty string.
// This prevents io.ReadAll from blocking on a tty waiting for Ctrl-D.
// Any Stat() error is treated as "not piped" — conservative to avoid hangs.
func readStdinIfPiped() (string, error) {
	info, err := os.Stdin.Stat()
	if err != nil || info.Mode()&os.ModeCharDevice != 0 {
		return "", nil
	}
	data, err := io.ReadAll(os.Stdin)
	if err != nil {
		return "", fmt.Errorf("read stdin: %w", err)
	}
	return strings.TrimRight(string(data), "\n"), nil
}
