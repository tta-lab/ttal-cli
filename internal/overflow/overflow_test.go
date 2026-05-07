package overflow

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestWrite_belowThreshold(t *testing.T) {
	dir := t.TempDir()
	body := "short message"
	got := Write(body, DefaultThreshold, dir)
	if got != body {
		t.Errorf("Write() = %q, want %q", got, body)
	}
}

func TestWrite_aboveThreshold(t *testing.T) {
	dir := t.TempDir()
	body := strings.Repeat("x", DefaultThreshold+1)
	got := Write(body, DefaultThreshold, dir)
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
	if !strings.Contains(got, dir) {
		t.Errorf("Write() should reference the correct overflow dir, got %q", got)
	}
}

func TestWrite_fallbackOnUnwritableDir(t *testing.T) {
	body := strings.Repeat("y", DefaultThreshold+1)
	got := Write(body, DefaultThreshold, "/root/overflow-test")
	if got == body {
		t.Fatal("Write() should still truncate when overflow dir is unwritable")
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

func TestWrite_writesFile(t *testing.T) {
	dir := t.TempDir()
	body := strings.Repeat("z", DefaultThreshold+1)
	_ = Write(body, DefaultThreshold, dir)
	files, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("ReadDir: %v", err)
	}
	if len(files) != 1 {
		t.Fatalf("expected 1 overflow file, got %d", len(files))
	}
	if !strings.Contains(files[0].Name(), "overflow_") {
		t.Errorf("expected overflow_ prefix, got %s", files[0].Name())
	}
}
