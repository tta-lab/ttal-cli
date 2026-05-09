package owner

import (
	"os"
	"testing"

	"github.com/tta-lab/ttal-cli/internal/taskwarrior"
)

func TestResolveOwner_WorkerPlane(t *testing.T) {
	t.Setenv("TTAL_JOB_ID", "227b31a9")
	t.Setenv("TTAL_AGENT_NAME", "coder")

	origFn := ExportTaskByHexIDFn
	ExportTaskByHexIDFn = func(_ string, _ string) (*taskwarrior.Task, error) {
		return &taskwarrior.Task{UUID: "227b31a9-bb81-4862-9122-893a1698c127", Owner: "kestrel"}, nil
	}
	defer func() { ExportTaskByHexIDFn = origFn }()

	got := ResolveOwner()
	if got != "kestrel" {
		t.Errorf("ResolveOwner() = %q, want %q", got, "kestrel")
	}
}

func TestResolveOwner_WorkerNoOwner(t *testing.T) {
	t.Setenv("TTAL_JOB_ID", "227b31a9")

	origFn := ExportTaskByHexIDFn
	ExportTaskByHexIDFn = func(_ string, _ string) (*taskwarrior.Task, error) {
		return &taskwarrior.Task{UUID: "227b31a9-bb81-4862-9122-893a1698c127", Owner: ""}, nil
	}
	defer func() { ExportTaskByHexIDFn = origFn }()

	got := ResolveOwner()
	if got != FallbackOwner {
		t.Errorf("ResolveOwner() = %q, want %q", got, FallbackOwner)
	}
}

func TestResolveOwner_WorkerExportError(t *testing.T) {
	t.Setenv("TTAL_JOB_ID", "deadbeef")

	origFn := ExportTaskByHexIDFn
	ExportTaskByHexIDFn = func(_ string, _ string) (*taskwarrior.Task, error) {
		return nil, os.ErrNotExist
	}
	defer func() { ExportTaskByHexIDFn = origFn }()

	got := ResolveOwner()
	if got != FallbackOwner {
		t.Errorf("ResolveOwner() = %q, want %q", got, FallbackOwner)
	}
}

func TestResolveOwner_ManagerPlane(t *testing.T) {
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

	got := ResolveOwner()
	if got != "neil" {
		t.Errorf("ResolveOwner() = %q, want %q", got, "neil")
	}
}

func TestResolveOwner_ManagerNoAdmin(t *testing.T) {
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
	// No humans.toml — adminHuman will be nil.

	got := ResolveOwner()
	if got != FallbackOwner {
		t.Errorf("ResolveOwner() = %q, want %q", got, FallbackOwner)
	}
}

func TestResolveOwner_ConfigLoadError(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)
	t.Setenv("TTAL_JOB_ID", "")
	t.Setenv("TTAL_AGENT_NAME", "astra")
	// No config.toml at all — config.Load will fail.

	got := ResolveOwner()
	if got != FallbackOwner {
		t.Errorf("ResolveOwner() = %q, want %q", got, FallbackOwner)
	}
}
