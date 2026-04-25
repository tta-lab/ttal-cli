package cmd

import (
	"os"
	"strings"
	"testing"
	"time"

	"github.com/tta-lab/ttal-cli/internal/daemon"
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

func TestSendCmd_NoEnvSendsAsSystem(t *testing.T) {
	calls := make([]daemon.SendRequest, 0)
	orig := daemonSendFn
	daemonSendFn = func(req daemon.SendRequest) error {
		calls = append(calls, req)
		return nil
	}
	t.Cleanup(func() { daemonSendFn = orig })

	sendTo = "neil"
	t.Setenv("TTAL_AGENT_NAME", "")
	t.Setenv("TTAL_JOB_ID", "")

	// Capture args for RunE
	cmd := sendCmd
	args := []string{"hello from bare shell"}
	oldArgs := os.Args
	os.Args = append([]string{"ttal"}, args...)
	defer func() { os.Args = oldArgs }()

	err := cmd.RunE(cmd, args)
	if err != nil {
		t.Fatalf("sendCmd.RunE: %v", err)
	}
	if len(calls) != 1 {
		t.Fatalf("expected 1 call, got %d", len(calls))
	}
	if calls[0].From != "system" {
		t.Errorf("From = %q, want %q", calls[0].From, "system")
	}
	if calls[0].To != "neil" {
		t.Errorf("To = %q, want %q", calls[0].To, "neil")
	}
	if calls[0].Message != "hello from bare shell" {
		t.Errorf("Message = %q, want %q", calls[0].Message, "hello from bare shell")
	}
}
