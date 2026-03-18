package cmd

import (
	"bytes"
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
		lines := make([]string, 15)
		for i := range lines {
			lines[i] = "line"
		}
		input := strings.Join(lines, "\n")
		result := truncateOutput(input)
		if !strings.Contains(result, "... (5 more lines)") {
			t.Errorf("expected truncation message, got %q", result)
		}
		resultLines := strings.Split(result, "\n")
		if len(resultLines) != 11 { // 10 lines + 1 truncation message
			t.Errorf("expected 11 lines, got %d", len(resultLines))
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

func captureStderr(f func()) string {
	origStderr := os.Stderr
	r, w, _ := os.Pipe()
	os.Stderr = w

	f()

	w.Close()
	os.Stderr = origStderr
	var buf bytes.Buffer
	io.Copy(&buf, r) //nolint:errcheck
	return buf.String()
}

func TestRenderCommandResult_NonZeroExit(t *testing.T) {
	out := captureStderr(func() {
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
	out := captureStderr(func() {
		renderCommandResult("some output", 0)
	})
	if strings.Contains(out, "exit") {
		t.Errorf("expected no exit line for zero exit, got %q", out)
	}
	if !strings.Contains(out, "some output") {
		t.Errorf("expected output text in result, got %q", out)
	}
}

func TestRenderCommandResult_EmptyOutput(t *testing.T) {
	out := captureStderr(func() {
		renderCommandResult("", 0)
	})
	if strings.TrimSpace(out) != "" {
		t.Errorf("expected no output for empty+zero exit, got %q", out)
	}
}

func TestRenderCommandResult_EmptyOutputNonZeroExit(t *testing.T) {
	out := captureStderr(func() {
		renderCommandResult("", 2)
	})
	if !strings.Contains(out, "exit 2") {
		t.Errorf("expected 'exit 2' in output, got %q", out)
	}
}
