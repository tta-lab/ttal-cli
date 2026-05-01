package taskwarrior

import (
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestResolvePowerSyncDBPath(t *testing.T) {
	if _, err := exec.LookPath("task"); err != nil {
		t.Skip("task binary not on PATH")
	}
	p, err := ResolvePowerSyncDBPath()
	if err != nil {
		t.Skipf("powersync.db_path not configured (likely non-fork taskwarrior): %v", err)
	}
	if p == "" {
		t.Fatal("got empty path")
	}
	if !filepath.IsAbs(p) {
		t.Fatalf("expected absolute path, got %q", p)
	}
	if strings.HasPrefix(p, "~/") {
		t.Fatalf("expected ~ to be expanded, got %q", p)
	}
}
