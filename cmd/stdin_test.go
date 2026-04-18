package cmd

import (
	"os"
	"testing"
)

// restoreStdin returns a function to restore os.Stdin to its original value.
// Call via defer: defer restoreStdin(oldStdin)()
func restoreStdin(old *os.File) func() {
	return func() {
		os.Stdin = old
	}
}

func TestReadStdinIfPiped_NonEmpty(t *testing.T) {
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("pipe: %v", err)
	}
	old := os.Stdin
	defer restoreStdin(old)()
	os.Stdin = r
	defer r.Close()

	if _, err := w.WriteString("hello\n"); err != nil {
		t.Fatalf("write: %v", err)
	}
	w.Close()

	got, err := readStdinIfPiped()
	if err != nil {
		t.Fatalf("readStdinIfPiped: %v", err)
	}
	if got != "hello" {
		t.Errorf("got %q, want %q", got, "hello")
	}
}

func TestReadStdinIfPiped_Multiline(t *testing.T) {
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("pipe: %v", err)
	}
	old := os.Stdin
	defer restoreStdin(old)()
	os.Stdin = r
	defer r.Close()

	if _, err := w.WriteString("a\nb\nc\n"); err != nil {
		t.Fatalf("write: %v", err)
	}
	w.Close()

	got, err := readStdinIfPiped()
	if err != nil {
		t.Fatalf("readStdinIfPiped: %v", err)
	}
	if got != "a\nb\nc" {
		t.Errorf("got %q, want %q", got, "a\nb\nc")
	}
}

func TestReadStdinIfPiped_Empty(t *testing.T) {
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("pipe: %v", err)
	}
	old := os.Stdin
	defer restoreStdin(old)()
	os.Stdin = r
	defer r.Close()
	w.Close()

	got, err := readStdinIfPiped()
	if err != nil {
		t.Fatalf("readStdinIfPiped: %v", err)
	}
	if got != "" {
		t.Errorf("got %q, want %q", got, "")
	}
}

func TestReadStdinIfPiped_TrailingBlanks(t *testing.T) {
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("pipe: %v", err)
	}
	old := os.Stdin
	defer restoreStdin(old)()
	os.Stdin = r
	defer r.Close()

	if _, err := w.WriteString("hi\n\n\n"); err != nil {
		t.Fatalf("write: %v", err)
	}
	w.Close()

	got, err := readStdinIfPiped()
	if err != nil {
		t.Fatalf("readStdinIfPiped: %v", err)
	}
	if got != "hi" {
		t.Errorf("got %q, want %q", got, "hi")
	}
}
