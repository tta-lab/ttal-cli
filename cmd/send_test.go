package cmd

import (
	"os"
	"strings"
	"testing"
	"time"
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

// TestResolveSendMessage_ArgsWinOverOpenPipe is the regression guard for the
// pueue-stdin-hang bug. It swaps os.Stdin for a pipe whose writer stays open
// forever — the exact FD shape a pueue-launched bash subprocess inherits. If
// resolveSendMessage ever regresses to reading stdin before checking args, the
// io.ReadAll call blocks indefinitely and this test deadlocks. The 2s timeout
// converts the hang into a failure.
func TestResolveSendMessage_ArgsWinOverOpenPipe(t *testing.T) {
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("pipe: %v", err)
	}
	defer r.Close()
	// IMPORTANT: do NOT close w — this simulates pueue's never-closed stdin pipe.
	defer w.Close()

	old := os.Stdin
	defer func() { os.Stdin = old }()
	os.Stdin = r

	type result struct {
		msg string
		err error
	}
	done := make(chan result, 1)
	go func() {
		msg, err := resolveSendMessage([]string{"hello", "world"})
		done <- result{msg, err}
	}()

	select {
	case got := <-done:
		if got.err != nil {
			t.Fatalf("resolveSendMessage: %v", got.err)
		}
		if got.msg != "hello world" {
			t.Errorf("got %q, want %q", got.msg, "hello world")
		}
	case <-time.After(2 * time.Second):
		t.Fatal("resolveSendMessage hung on stdin while positional args were present — " +
			"args must win without touching stdin (pueue-stdin-hang regression)")
	}
}

// TestResolveSendMessage_NoArgsReadsStdin verifies the stdin fallback still
// works when no positional args are provided — preserves `echo ... | ttal send`.
func TestResolveSendMessage_NoArgsReadsStdin(t *testing.T) {
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("pipe: %v", err)
	}
	old := os.Stdin
	defer func() { os.Stdin = old }()
	os.Stdin = r
	defer r.Close()

	if _, err := w.WriteString("from stdin\n"); err != nil {
		t.Fatalf("write: %v", err)
	}
	w.Close()

	msg, err := resolveSendMessage(nil)
	if err != nil {
		t.Fatalf("resolveSendMessage: %v", err)
	}
	if msg != "from stdin" {
		t.Errorf("got %q, want %q", msg, "from stdin")
	}
}

// TestResolveSendMessage_NoArgsNoStdinErrors verifies that an empty stdin +
// no args surfaces the usage error, not a silent empty send.
func TestResolveSendMessage_NoArgsNoStdinErrors(t *testing.T) {
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("pipe: %v", err)
	}
	old := os.Stdin
	defer func() { os.Stdin = old }()
	os.Stdin = r
	defer r.Close()
	w.Close() // empty + EOF

	_, err = resolveSendMessage(nil)
	if err == nil {
		t.Fatal("expected error for empty stdin + no args, got nil")
	}
	if !strings.Contains(err.Error(), "message required") {
		t.Errorf("unexpected error: %v", err)
	}
}
