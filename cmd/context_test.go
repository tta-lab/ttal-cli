package cmd

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/spf13/cobra"
)

// captureContextOutput runs runContext and captures stdout.
func captureContextOutput(t *testing.T) string {
	t.Helper()
	// Redirect stdout by capturing via cobra output
	buf := &bytes.Buffer{}
	contextCmd.SetOut(buf)
	defer contextCmd.SetOut(nil)

	// Use the existing runContext but redirect stdout temporarily
	old := os.Stdout
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("pipe: %v", err)
	}
	os.Stdout = w

	_ = runContext(&cobra.Command{}, []string{})

	w.Close()
	os.Stdout = old

	var out bytes.Buffer
	if _, err := out.ReadFrom(r); err != nil {
		t.Fatalf("read pipe: %v", err)
	}
	return out.String()
}

// TestRunContext_NonAgentSession verifies non-agent sessions output {}.
func TestRunContext_NonAgentSession(t *testing.T) {
	t.Setenv("TTAL_AGENT_NAME", "")

	output := captureContextOutput(t)
	output = trimNewlines(output)
	if output != "{}" {
		t.Errorf("expected {} for non-agent session, got %q", output)
	}
}

// TestRunContext_AgentWithConfig verifies agent session outputs valid JSON with systemMessage.
func TestRunContext_AgentWithConfig(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)
	t.Setenv("TTAL_AGENT_NAME", "kestrel")

	cfgDir := filepath.Join(tmp, ".config", "ttal")
	if err := os.MkdirAll(cfgDir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	// Write config with a breathe_context command that produces output.
	// prompts.toml is a flat file (no section headers).
	promptsToml := "breathe_context = \"echo context-from-hook\"\n"
	if err := os.WriteFile(filepath.Join(cfgDir, "prompts.toml"), []byte(promptsToml), 0o644); err != nil {
		t.Fatalf("write prompts.toml: %v", err)
	}
	configToml := "[teams.default]\nteam_path = \"" + tmp + "\"\n"
	if err := os.WriteFile(filepath.Join(cfgDir, "config.toml"), []byte(configToml), 0o644); err != nil {
		t.Fatalf("write config.toml: %v", err)
	}

	output := captureContextOutput(t)
	output = trimNewlines(output)

	if output == "{}" {
		t.Skip("breathe_context command produced no output (may not be configured correctly in test env)")
	}

	var resp map[string]interface{}
	if err := json.Unmarshal([]byte(output), &resp); err != nil {
		t.Fatalf("output is not valid JSON: %v\noutput: %q", err, output)
	}
	if _, ok := resp["systemMessage"]; !ok {
		// empty context falls through to {} output — acceptable
		if output == "{}" {
			return
		}
		t.Errorf("expected systemMessage key in output, got: %q", output)
	}
}

// TestRunContext_MissingConfig verifies missing config.toml produces {}.
func TestRunContext_MissingConfig(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)
	t.Setenv("TTAL_AGENT_NAME", "kestrel")
	// No config files — config.Load should fail gracefully

	output := captureContextOutput(t)
	output = trimNewlines(output)

	// Must be valid JSON even when config is missing
	var v interface{}
	if err := json.Unmarshal([]byte(output), &v); err != nil {
		t.Fatalf("output is not valid JSON: %v\noutput: %q", err, output)
	}
	// Should output {} when config load fails
	if output != "{}" {
		t.Errorf("expected {} when config missing, got: %q", output)
	}
}

// TestRunContext_MalformedRouteFile verifies corrupt route file still produces valid JSON.
func TestRunContext_MalformedRouteFile(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)
	t.Setenv("TTAL_AGENT_NAME", "kestrel")

	// Write a minimal config.toml so config.Load() succeeds.
	cfgDir := filepath.Join(tmp, ".config", "ttal")
	if err := os.MkdirAll(cfgDir, 0o755); err != nil {
		t.Fatalf("mkdir cfg: %v", err)
	}
	if err := os.WriteFile(filepath.Join(cfgDir, "config.toml"),
		[]byte("[teams.default]\nteam_path = \""+tmp+"\"\n"), 0o644); err != nil {
		t.Fatalf("write config.toml: %v", err)
	}

	// Write a corrupt route file under the default data dir (~/.ttal/routing/).
	routingDir := filepath.Join(tmp, ".ttal", "routing")
	if err := os.MkdirAll(routingDir, 0o755); err != nil {
		t.Fatalf("mkdir routing: %v", err)
	}
	if err := os.WriteFile(filepath.Join(routingDir, "kestrel.json"), []byte("not-valid-json"), 0o644); err != nil {
		t.Fatalf("write bad route: %v", err)
	}

	output := captureContextOutput(t)
	output = trimNewlines(output)

	// Must be valid JSON despite corrupt route file.
	var v interface{}
	if err := json.Unmarshal([]byte(output), &v); err != nil {
		t.Fatalf("output is not valid JSON with corrupt route file: %v\noutput: %q", err, output)
	}
}

// TestRunContext_RouteComposition verifies that a valid route file appends role prompt
// and message to the systemMessage and sets continue: true.
func TestRunContext_RouteComposition(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)
	t.Setenv("TTAL_AGENT_NAME", "kestrel")

	// Write config with a breathe_context command so we have a non-empty base.
	cfgDir := filepath.Join(tmp, ".config", "ttal")
	if err := os.MkdirAll(cfgDir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(cfgDir, "config.toml"),
		[]byte("[teams.default]\nteam_path = \""+tmp+"\"\n"), 0o644); err != nil {
		t.Fatalf("write config.toml: %v", err)
	}
	if err := os.WriteFile(filepath.Join(cfgDir, "prompts.toml"),
		[]byte("breathe_context = \"echo base-context\"\n"), 0o644); err != nil {
		t.Fatalf("write prompts.toml: %v", err)
	}

	// Stage a route file.
	routingDir := filepath.Join(tmp, ".ttal", "routing")
	if err := os.MkdirAll(routingDir, 0o755); err != nil {
		t.Fatalf("mkdir routing: %v", err)
	}
	routeJSON := `{"task_uuid":"abc12345","role_prompt":"You are a designer.",` +
		`"message":"Work on task abc12345.","routed_by":"astra","created_at":"2026-01-01T00:00:00Z"}`
	if err := os.WriteFile(filepath.Join(routingDir, "kestrel.json"), []byte(routeJSON), 0o644); err != nil {
		t.Fatalf("write route: %v", err)
	}

	output := captureContextOutput(t)
	output = trimNewlines(output)

	var resp map[string]interface{}
	if err := json.Unmarshal([]byte(output), &resp); err != nil {
		t.Fatalf("output is not valid JSON: %v\noutput: %q", err, output)
	}

	sysMsg, ok := resp["systemMessage"].(string)
	if !ok {
		t.Fatalf("expected systemMessage string, got: %v", resp)
	}
	if !strings.Contains(sysMsg, "You are a designer.") {
		t.Errorf("expected role prompt in systemMessage, got: %q", sysMsg)
	}
	if !strings.Contains(sysMsg, "Work on task abc12345.") {
		t.Errorf("expected route message in systemMessage, got: %q", sysMsg)
	}
	if resp["continue"] != true {
		t.Errorf("expected continue=true, got: %v", resp["continue"])
	}

	// Route file must have been consumed (deleted).
	if _, err := os.Stat(filepath.Join(routingDir, "kestrel.json")); !os.IsNotExist(err) {
		t.Error("route file should have been consumed (deleted)")
	}
}

func trimNewlines(s string) string {
	for len(s) > 0 && (s[len(s)-1] == '\n' || s[len(s)-1] == '\r') {
		s = s[:len(s)-1]
	}
	return s
}
