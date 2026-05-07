package overflow

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestWrite_belowThreshold(t *testing.T) {
	body := "short message"
	got := Write(body, DefaultThreshold)
	if got != body {
		t.Errorf("Write() = %q, want %q", got, body)
	}
}

func TestWrite_aboveThreshold(t *testing.T) {
	body := strings.Repeat("x", DefaultThreshold+1)
	got := Write(body, DefaultThreshold)
	if got == body {
		t.Fatal("Write() returned original body for oversize message")
	}
	if !strings.HasPrefix(got, "xx") {
		t.Errorf("Write() preview should start with body content, got %q", got)
	}
	if !strings.Contains(got, "[message truncated") {
		t.Errorf("Write() should contain truncation notice, got %q", got)
	}
	if !strings.Contains(got, "overflow_") {
		t.Errorf("Write() should reference an overflow file, got %q", got)
	}
	if strings.Contains(got, "~~") {
		t.Errorf("Write() should not contain literal ~~ (home dir marker): %q", got)
	}
}

func TestTruncate_underLimit(t *testing.T) {
	body := "short"
	got := truncate(body)
	if got != body {
		t.Errorf("truncate(%q) = %q, want %q", body, got, body)
	}
}

func TestTruncate_overLimit(t *testing.T) {
	body := strings.Repeat("a", 300)
	got := truncate(body)
	if len(got) != 200 {
		t.Errorf("truncate() length = %d, want 200", len(got))
	}
	if got != strings.Repeat("a", 200) {
		t.Errorf("truncate() returned wrong content")
	}
}

func TestMakePreview(t *testing.T) {
	body := strings.Repeat("b", 300)
	preview := truncate(body)
	path := filepath.Join("/tmp", "test_overflow.md")
	got := makePreview(body, path)
	if !strings.Contains(got, preview) {
		t.Errorf("makePreview() should contain the preview text")
	}
	if !strings.Contains(got, path) {
		t.Errorf("makePreview() should contain the file path")
	}
}

func TestMustExpandDir_noTilde(t *testing.T) {
	got := mustExpandDir("/tmp/foo")
	if got != "/tmp/foo" {
		t.Errorf("mustExpandDir(%q) = %q, want %q", "/tmp/foo", got, "/tmp/foo")
	}
}

func TestMustExpandDir_withTilde(t *testing.T) {
	got := mustExpandDir("~/.ttal/overflow")
	if !strings.HasPrefix(got, "/") || !strings.Contains(got, ".ttal/overflow") {
		t.Errorf("mustExpandDir() = %q, should expand ~ to home dir", got)
	}
	if strings.Contains(got, "~~") {
		t.Errorf("mustExpandDir() should not double-expand ~: %q", got)
	}
}
