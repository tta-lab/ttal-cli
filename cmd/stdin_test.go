package cmd

import (
	"os"
	"testing"
)

func TestReadStdinIfPiped_NonEmpty(t *testing.T) {
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("pipe: %v", err)
	}
	orig := os.Stdin
	defer func() { os.Stdin = orig }()
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
	orig := os.Stdin
	defer func() { os.Stdin = orig }()
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
}

func TestReadStdinIfPiped_TrailingBlanks(t *testing.T) {
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("pipe: %v", err)
	}
	orig := os.Stdin
	defer func() { os.Stdin = orig }()
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
