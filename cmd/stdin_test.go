package cmd

import (
	"io"
	"os"
	"strings"
	"testing"
)

func TestReadStdinIfPiped(t *testing.T) {
	t.Run("piped_non_empty", func(t *testing.T) {
		r, w, err := os.Pipe()
		if err != nil {
			t.Fatalf("pipe: %v", err)
		}
		orig := os.Stdin
		defer func() { os.Stdin = orig }()
		os.Stdin = r
		defer r.Close()

		_, err = w.WriteString("hello\n")
		if err != nil {
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
	})

	t.Run("piped_multiline", func(t *testing.T) {
		r, w, err := os.Pipe()
		if err != nil {
			t.Fatalf("pipe: %v", err)
		}
		orig := os.Stdin
		defer func() { os.Stdin = orig }()
		os.Stdin = r
		defer r.Close()

		_, err = w.WriteString("a\nb\nc\n")
		if err != nil {
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
	})

	t.Run("piped_empty", func(t *testing.T) {
		r, w, err := os.Pipe()
		if err != nil {
			t.Fatalf("pipe: %v", err)
		}
		orig := os.Stdin
		defer func() { os.Stdin = orig }()
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
	})

	t.Run("piped_trailing_blanks", func(t *testing.T) {
		r, w, err := os.Pipe()
		if err != nil {
			t.Fatalf("pipe: %v", err)
		}
		orig := os.Stdin
		defer func() { os.Stdin = orig }()
		os.Stdin = r
		defer r.Close()

		_, err = w.WriteString("hi\n\n\n")
		if err != nil {
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
	})

	// TTY case skipped (needs pty) — covered by inspection.
	_ = io.Discard
	_ = strings.Compare
}
