package cmd

import (
	"os"
	"strings"
	"testing"
	"time"

	"github.com/tta-lab/ttal-cli/internal/owner"
	"github.com/tta-lab/ttal-cli/internal/sendfmt"
	"github.com/tta-lab/ttal-cli/internal/taskwarrior"
)

func TestWakeCmd_ManagerPlane(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)
	t.Setenv("TTAL_JOB_ID", "")
	t.Setenv("TTAL_AGENT_NAME", "astra")

	cfgDir := tmp + "/.config/ttal"
	if err := os.MkdirAll(cfgDir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(cfgDir+"/config.toml",
		[]byte("[teams.default]\nteam_path = \""+tmp+"\"\n"), 0o644); err != nil {
		t.Fatalf("write config: %v", err)
	}
	if err := os.WriteFile(cfgDir+"/humans.toml",
		[]byte("[neil]\nname = \"Neil\"\ntelegram_chat_id = \"12345\"\nadmin = true\n"), 0o644); err != nil {
		t.Fatalf("write humans: %v", err)
	}

	// Use deterministic time
	restore := sendfmt.SetNowForTest(func() time.Time {
		return time.Date(2026, 5, 9, 14, 30, 0, 0, time.Local)
	})
	defer restore()

	out := captureStdout(t, func() {
		_ = wakeCmd.RunE(wakeCmd, nil)
	})

	if !strings.Contains(out, "[telegram from:neil] [14:30:00]") {
		t.Errorf("expected manager prefix in output, got: %q", out)
	}
	if !strings.Contains(out, "ttal send --to neil") {
		t.Errorf("expected reply hint with neil, got: %q", out)
	}
	if !strings.Contains(out, "ttal context") {
		t.Errorf("expected trigger message, got: %q", out)
	}
}

func TestWakeCmd_WorkerPlane(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)
	t.Setenv("TTAL_JOB_ID", "227b31a9")
	t.Setenv("TTAL_AGENT_NAME", "coder")

	origFn := owner.ExportTaskByHexIDFn
	owner.ExportTaskByHexIDFn = func(_ string, _ string) (*taskwarrior.Task, error) {
		return &taskwarrior.Task{UUID: "227b31a9-bb81-4862-9122-893a1698c127", Owner: "kestrel"}, nil
	}
	defer func() { owner.ExportTaskByHexIDFn = origFn }()

	restore := sendfmt.SetNowForTest(func() time.Time {
		return time.Date(2026, 5, 9, 10, 15, 0, 0, time.Local)
	})
	defer restore()

	out := captureStdout(t, func() {
		_ = wakeCmd.RunE(wakeCmd, nil)
	})

	if !strings.Contains(out, "[telegram from:kestrel] [10:15:00]") {
		t.Errorf("expected worker prefix in output, got: %q", out)
	}
	if !strings.Contains(out, "ttal send --to kestrel") {
		t.Errorf("expected reply hint with kestrel, got: %q", out)
	}
}

func TestWakeCmd_FallbackToSystem(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)
	t.Setenv("TTAL_JOB_ID", "")
	t.Setenv("TTAL_AGENT_NAME", "astra")
	// No config — owner resolution falls back to "system"

	restore := sendfmt.SetNowForTest(func() time.Time {
		return time.Date(2026, 5, 9, 12, 0, 0, 0, time.Local)
	})
	defer restore()

	out := captureStdout(t, func() {
		_ = wakeCmd.RunE(wakeCmd, nil)
	})

	if !strings.Contains(out, "[telegram from:system] [12:00:00]") {
		t.Errorf("expected system fallback prefix, got: %q", out)
	}
	if !strings.Contains(out, "ttal send --to system") {
		t.Errorf("expected reply hint with system, got: %q", out)
	}
}

func TestWakeCmd_OutputShape(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)
	t.Setenv("TTAL_JOB_ID", "")
	t.Setenv("TTAL_AGENT_NAME", "astra")

	cfgDir := tmp + "/.config/ttal"
	if err := os.MkdirAll(cfgDir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(cfgDir+"/config.toml",
		[]byte("[teams.default]\nteam_path = \""+tmp+"\"\n"), 0o644); err != nil {
		t.Fatalf("write config: %v", err)
	}
	if err := os.WriteFile(cfgDir+"/humans.toml",
		[]byte("[neil]\nname = \"Neil\"\ntelegram_chat_id = \"12345\"\nadmin = true\n"), 0o644); err != nil {
		t.Fatalf("write humans: %v", err)
	}

	restore := sendfmt.SetNowForTest(func() time.Time {
		return time.Date(2026, 5, 9, 8, 5, 30, 0, time.Local)
	})
	defer restore()

	out := captureStdout(t, func() {
		_ = wakeCmd.RunE(wakeCmd, nil)
	})

	expectedLines := []string{
		"[telegram from:neil] [08:05:30] Run `ttal context` for your briefing, then act on the role prompt.",
		"<i>--- Reply with:",
		"cat <<'EOF' | ttal send --to neil",
		"your message",
		"EOF</i>",
	}
	for _, line := range expectedLines {
		if !strings.Contains(out, line) {
			t.Errorf("expected output to contain %q, got: %q", line, out)
		}
	}
}
