package tmux

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestEnvDefaultsTmuxTmpdirFromXDGRuntimeDir(t *testing.T) {
	runtimeDir := t.TempDir()
	t.Setenv("XDG_RUNTIME_DIR", runtimeDir)
	t.Setenv("TMUX_TMPDIR", "")
	t.Setenv("TMUX", filepath.Join(runtimeDir, "tmux-1000", "default")+",123,0")

	env := IsolatedEnv()
	want := filepath.Join(runtimeDir, "ttal-tmux")

	if got := envValue(env, "TMUX_TMPDIR"); got != want {
		t.Fatalf("TMUX_TMPDIR = %q, want %q", got, want)
	}
	if got := envCount(env, "TMUX_TMPDIR"); got != 1 {
		t.Fatalf("TMUX_TMPDIR entry count = %d, want 1", got)
	}
	if got := envValue(env, "TMUX"); got != "" {
		t.Fatalf("TMUX = %q, want empty", got)
	}
	if info, err := os.Stat(want); err != nil {
		t.Fatalf("expected tmux tmpdir to be created: %v", err)
	} else if !info.IsDir() {
		t.Fatalf("expected tmux tmpdir to be a directory")
	}
}

func TestEnvIgnoresAmbientTmuxTmpdir(t *testing.T) {
	runtimeDir := t.TempDir()
	ambient := filepath.Join(t.TempDir(), "ambient-tmux")
	t.Setenv("XDG_RUNTIME_DIR", runtimeDir)
	t.Setenv("TMUX_TMPDIR", ambient)

	env := IsolatedEnv()
	want := filepath.Join(runtimeDir, "ttal-tmux")

	if got := envValue(env, "TMUX_TMPDIR"); got != want {
		t.Fatalf("TMUX_TMPDIR = %q, want %q", got, want)
	}
	if _, err := os.Stat(want); err != nil {
		t.Fatalf("expected ttal tmux tmpdir to be created: %v", err)
	}
}

func TestEnvAllowsTtalSpecificTmpdirOverride(t *testing.T) {
	runtimeDir := t.TempDir()
	override := filepath.Join(t.TempDir(), "ttal-custom-tmux")
	t.Setenv("XDG_RUNTIME_DIR", runtimeDir)
	t.Setenv("TMUX_TMPDIR", filepath.Join(t.TempDir(), "ambient-tmux"))
	t.Setenv("TTAL_TMUX_TMPDIR", override)

	env := IsolatedEnv()

	if got := envValue(env, "TMUX_TMPDIR"); got != override {
		t.Fatalf("TMUX_TMPDIR = %q, want %q", got, override)
	}
	if _, err := os.Stat(filepath.Join(runtimeDir, "ttal-tmux")); !os.IsNotExist(err) {
		t.Fatalf("default tmux tmpdir should not be created when TTAL_TMUX_TMPDIR is set")
	}
}

func envValue(env []string, key string) string {
	prefix := key + "="
	for i := len(env) - 1; i >= 0; i-- {
		if strings.HasPrefix(env[i], prefix) {
			return strings.TrimPrefix(env[i], prefix)
		}
	}
	return ""
}

func envCount(env []string, key string) int {
	prefix := key + "="
	var count int
	for _, part := range env {
		if strings.HasPrefix(part, prefix) {
			count++
		}
	}
	return count
}
