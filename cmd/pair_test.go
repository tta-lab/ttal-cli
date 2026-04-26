package cmd

import (
	"os"
	"strings"
	"testing"

	"github.com/tta-lab/ttal-cli/internal/taskwarrior"
)

func TestPairCmd_WorkerSession(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)
	t.Setenv("TTAL_JOB_ID", "227b31a9")
	t.Setenv("TTAL_AGENT_NAME", "coder")

	// Inject a fake task lookup.
	origFn := exportTaskByHexIDFn
	exportTaskByHexIDFn = func(_ string, _ string) (*taskwarrior.Task, error) {
		return &taskwarrior.Task{UUID: "227b31a9-bb81-4862-9122-893a1698c127", Owner: "kestrel"}, nil
	}
	defer func() { exportTaskByHexIDFn = origFn }()

	out := captureStdout(t, func() {
		_ = pairCmd.RunE(pairCmd, nil)
	})

	if !strings.Contains(out, "Pairing with **kestrel**") {
		t.Errorf("expected 'Pairing with **kestrel**' in output, got: %q", out)
	}
	if !strings.Contains(out, "ttal send --to kestrel") {
		t.Errorf("expected send line in output, got: %q", out)
	}
}

func TestPairCmd_ManagerSession(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)
	t.Setenv("TTAL_JOB_ID", "")
	t.Setenv("TTAL_AGENT_NAME", "astra")

	cfgDir := tmp + "/.config/ttal"
	if err := os.MkdirAll(cfgDir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	config := "[teams.default]\nteam_path = \"" + tmp + "\"\n"
	if err := os.WriteFile(cfgDir+"/config.toml", []byte(config), 0o644); err != nil {
		t.Fatalf("write config: %v", err)
	}
	humans := "[neil]\nname = \"Neil\"\ntelegram_chat_id = \"12345\"\nadmin = true\n"
	if err := os.WriteFile(cfgDir+"/humans.toml", []byte(humans), 0o644); err != nil {
		t.Fatalf("write humans: %v", err)
	}

	out := captureStdout(t, func() {
		_ = pairCmd.RunE(pairCmd, nil)
	})

	if !strings.Contains(out, "Pairing with **neil**") {
		t.Errorf("expected 'Pairing with **neil**' in output, got: %q", out)
	}
}

func TestPairCmd_NoAdminHuman(t *testing.T) {
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

	out := captureStdout(t, func() {
		_ = pairCmd.RunE(pairCmd, nil)
	})

	if out != "" {
		t.Errorf("expected empty output with no admin, got: %q", out)
	}
}

func TestPairCmd_WorkerNoOwner(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)
	t.Setenv("TTAL_JOB_ID", "227b31a9")
	t.Setenv("TTAL_AGENT_NAME", "coder")

	origFn := exportTaskByHexIDFn
	exportTaskByHexIDFn = func(_ string, _ string) (*taskwarrior.Task, error) {
		return &taskwarrior.Task{UUID: "227b31a9-bb81-4862-9122-893a1698c127", Owner: ""}, nil
	}
	defer func() { exportTaskByHexIDFn = origFn }()

	out := captureStdout(t, func() {
		_ = pairCmd.RunE(pairCmd, nil)
	})

	if out != "" {
		t.Errorf("expected empty output with no owner, got: %q", out)
	}
}
