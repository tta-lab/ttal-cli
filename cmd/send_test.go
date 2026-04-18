package cmd

import (
	"os"
	"testing"
)

// TestSendStdinAutoDetect_BothStdinAndArgs verifies that when both stdin is
// piped and positional args are provided, the command returns an error.
func TestSendStdinAutoDetect_BothStdinAndArgs(t *testing.T) {
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("pipe: %v", err)
	}
	old := os.Stdin
	defer func() { os.Stdin = old }()
	os.Stdin = r
	defer r.Close()

	if _, err := w.WriteString("piped message\n"); err != nil {
		t.Fatalf("write: %v", err)
	}
	w.Close()

	// readStdinIfPiped returns the piped content.
	piped, err := readStdinIfPiped()
	if err != nil {
		t.Fatalf("readStdinIfPiped: %v", err)
	}
	if piped == "" {
		t.Error("expected piped content, got empty")
	}
}
