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

// TestResolveSendMessage_ArgsErrorWithoutTouchingOpenPipe verifies the current
// stdin-only contract: positional args are rejected immediately, before any
// stdin read is attempted. The still-open pipe is a guard against accidental
// regressions back to "args plus opportunistic stdin probing".
func TestResolveSendMessage_ArgsErrorWithoutTouchingOpenPipe(t *testing.T) {
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("pipe: %v", err)
	}
	defer r.Close()
	// IMPORTANT: do NOT close w — if resolveSendMessage touched stdin here, it
	// would block waiting for EOF and the test would fail on timeout.
	defer w.Close()

	old := os.Stdin
	defer func() { os.Stdin = old }()
	os.Stdin = r

	type result struct {
		err error
	}
	done := make(chan result, 1)
	go func() {
		_, err := resolveSendMessage([]string{"hello", "world"})
		done <- result{err: err}
	}()

	select {
	case got := <-done:
		if got.err == nil {
			t.Fatal("expected positional-args error, got nil")
		}
		if !strings.Contains(got.err.Error(), "reads stdin only") {
			t.Fatalf("expected stdin-only guidance, got: %v", got.err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("resolveSendMessage hung on stdin while positional args were present — " +
			"it must error before touching stdin")
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
	if !strings.Contains(err.Error(), "cat <<'END' | ttal send --to <name>") {
		t.Errorf("expected heredoc example in error, got: %v", err)
	}
	if strings.Contains(err.Error(), `ttal send --to kestrel "message"`) {
		t.Errorf("expected no positional-arg send example in error, got: %v", err)
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

	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("pipe: %v", err)
	}
	oldStdin := os.Stdin
	defer func() { os.Stdin = oldStdin }()
	os.Stdin = r
	defer r.Close()

	if _, err := w.WriteString("hello from bare shell\n"); err != nil {
		t.Fatalf("write: %v", err)
	}
	w.Close()

	cmd := sendCmd
	args := []string{}
	oldArgs := os.Args
	os.Args = append([]string{"ttal"}, args...)
	defer func() { os.Args = oldArgs }()

	err = cmd.RunE(cmd, args)
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
	if !calls[0].UserInitiated {
		t.Errorf("UserInitiated = false, want true (CLI must flag every ttal send)")
	}
}
