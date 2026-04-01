package cmd

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"strings"
	"testing"
)

func TestTruncateOutput(t *testing.T) {
	t.Run("under limit returned unchanged", func(t *testing.T) {
		lines := []string{"a", "b", "c", "d", "e"}
		input := strings.Join(lines, "\n")
		result := truncateOutput(input)
		if result != input {
			t.Errorf("expected unchanged, got %q", result)
		}
	})

	t.Run("exactly at limit returned unchanged", func(t *testing.T) {
		lines := []string{"a", "b", "c", "d", "e", "f", "g", "h", "i", "j"}
		input := strings.Join(lines, "\n")
		result := truncateOutput(input)
		if result != input {
			t.Errorf("expected unchanged, got %q", result)
		}
	})

	t.Run("over limit truncates with count", func(t *testing.T) {
		extra := 5
		totalLines := askMaxOutputLines + extra
		lines := make([]string, totalLines)
		for i := range lines {
			lines[i] = "line"
		}
		input := strings.Join(lines, "\n")
		result := truncateOutput(input)
		want := fmt.Sprintf("... (%d more lines)", extra)
		if !strings.Contains(result, want) {
			t.Errorf("expected truncation message %q, got %q", want, result)
		}
		resultLines := strings.Split(result, "\n")
		wantLineCount := askMaxOutputLines + 1 // truncated lines + suffix
		if len(resultLines) != wantLineCount {
			t.Errorf("expected %d lines, got %d", wantLineCount, len(resultLines))
		}
	})

	t.Run("empty string returned as-is", func(t *testing.T) {
		result := truncateOutput("")
		if result != "" {
			t.Errorf("expected empty, got %q", result)
		}
	})

	t.Run("single line returned unchanged", func(t *testing.T) {
		result := truncateOutput("hello")
		if result != "hello" {
			t.Errorf("expected %q, got %q", "hello", result)
		}
	})

	t.Run("trailing newlines stripped before counting", func(t *testing.T) {
		lines := make([]string, 5)
		for i := range lines {
			lines[i] = "line"
		}
		input := strings.Join(lines, "\n") + "\n\n"
		result := truncateOutput(input)
		// Should not be truncated — trailing newlines don't add lines
		if strings.Contains(result, "more lines") {
			t.Errorf("unexpected truncation for trailing-newline input: %q", result)
		}
	})
}

func captureStderr(t *testing.T, f func()) string {
	t.Helper()
	origStderr := os.Stderr
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal("os.Pipe:", err)
	}
	os.Stderr = w

	f()

	w.Close()
	os.Stderr = origStderr
	var buf bytes.Buffer
	if _, err := io.Copy(&buf, r); err != nil {
		t.Fatal("io.Copy from stderr pipe:", err)
	}
	return buf.String()
}

func TestRenderCommandStart(t *testing.T) {
	out := captureStderr(t, func() {
		renderCommandStart("ls -la")
	})
	if !strings.Contains(out, "$ ") {
		t.Errorf("expected '$ ' in output, got %q", out)
	}
	if !strings.Contains(out, "ls -la") {
		t.Errorf("expected command in output, got %q", out)
	}
}

func TestRenderCommandResult_NonZeroExit(t *testing.T) {
	out := captureStderr(t, func() {
		renderCommandResult("some output", 1)
	})
	if !strings.Contains(out, "exit 1") {
		t.Errorf("expected 'exit 1' in output, got %q", out)
	}
	if !strings.Contains(out, "some output") {
		t.Errorf("expected output text in result, got %q", out)
	}
}

func TestRenderCommandResult_ZeroExit(t *testing.T) {
	// With smart output, output is suppressed on success (exit 0)
	out := captureStderr(t, func() {
		renderCommandResult("some output", 0)
	})
	if strings.TrimSpace(out) != "" {
		t.Errorf("expected no output for zero exit (smart mode suppresses output on success), got %q", out)
	}
}

func TestRenderCommandResult_EmptyOutput(t *testing.T) {
	out := captureStderr(t, func() {
		renderCommandResult("", 0)
	})
	if strings.TrimSpace(out) != "" {
		t.Errorf("expected no output for empty+zero exit, got %q", out)
	}
}

func TestRenderCommandResult_EmptyOutputNonZeroExit(t *testing.T) {
	out := captureStderr(t, func() {
		renderCommandResult("", 2)
	})
	if !strings.Contains(out, "exit 2") {
		t.Errorf("expected 'exit 2' in output, got %q", out)
	}
}
