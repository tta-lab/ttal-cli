package temenos

import (
	"os"
	"strings"
	"testing"

	"github.com/tta-lab/ttal-cli/internal/taskwarrior"
)

func TestMCPConfig(t *testing.T) {
	got := MCPConfig(9783, "abc123")
	// Build expected string in parts to stay under line-length limit.
	want := `{"mcpServers":{"temenos":{"type":"http","url":"http://127.0.0.1:9783",` +
		`"headers":{"X-Session-Token":"abc123"}}}}`
	if got != want {
		t.Errorf("MCPConfig mismatch\nwant: %s\n got: %s", want, got)
	}
}

func TestMCPConfig_CustomPort(t *testing.T) {
	got := MCPConfig(9999, "deadbeef")
	want := `{"mcpServers":{"temenos":{"type":"http","url":"http://127.0.0.1:9999",` +
		`"headers":{"X-Session-Token":"deadbeef"}}}}`
	if got != want {
		t.Errorf("MCPConfig custom port failed: %s", got)
	}
}

func TestExtractToken_Found(t *testing.T) {
	annotations := []taskwarrior.Annotation{
		{Description: "plan: flicknote/abc123"},
		{Description: "temenos_token:my_secret_token"},
		{Description: "other annotation"},
	}
	got := ExtractToken(annotations)
	if got != "my_secret_token" {
		t.Errorf("expected my_secret_token, got %q", got)
	}
}

func TestExtractToken_NotFound(t *testing.T) {
	annotations := []taskwarrior.Annotation{
		{Description: "plan: flicknote/abc123"},
	}
	got := ExtractToken(annotations)
	if got != "" {
		t.Errorf("expected empty string, got %q", got)
	}
}

func TestExtractToken_Empty(t *testing.T) {
	got := ExtractToken(nil)
	if got != "" {
		t.Errorf("expected empty string for nil annotations, got %q", got)
	}
}

// --- MCP config file helpers ---

func TestWriteMCPConfigFile_RoundTrip(t *testing.T) {
	mcpJSON := MCPConfig(9783, "roundtriptoken")
	path, err := WriteMCPConfigFile("test-rt", mcpJSON)
	if err != nil {
		t.Fatalf("WriteMCPConfigFile: %v", err)
	}
	t.Cleanup(func() { _ = os.Remove(path) })

	if !strings.HasSuffix(path, "test-rt.json") {
		t.Errorf("unexpected path %q", path)
	}

	token := ReadMCPConfigToken("test-rt")
	if token != "roundtriptoken" {
		t.Errorf("ReadMCPConfigToken: got %q, want roundtriptoken", token)
	}
}

func TestReadMCPConfigToken_Missing(t *testing.T) {
	// File does not exist — should return empty string, not panic or error.
	got := ReadMCPConfigToken("test-missing-xyz")
	if got != "" {
		t.Errorf("expected empty string for missing file, got %q", got)
	}
}

func TestReadMCPConfigToken_InvalidJSON(t *testing.T) {
	path, err := WriteMCPConfigFile("test-badjson", "not-valid-json{{{")
	if err != nil {
		t.Fatalf("WriteMCPConfigFile: %v", err)
	}
	t.Cleanup(func() { _ = os.Remove(path) })

	got := ReadMCPConfigToken("test-badjson")
	if got != "" {
		t.Errorf("expected empty string for invalid JSON, got %q", got)
	}
}

func TestReadMCPConfigToken_NoTemenosKey(t *testing.T) {
	// Valid JSON but no "temenos" server entry.
	path, err := WriteMCPConfigFile("test-notemenos", `{"mcpServers":{}}`)
	if err != nil {
		t.Fatalf("WriteMCPConfigFile: %v", err)
	}
	t.Cleanup(func() { _ = os.Remove(path) })

	got := ReadMCPConfigToken("test-notemenos")
	if got != "" {
		t.Errorf("expected empty string when temenos key absent, got %q", got)
	}
}

func TestDeleteMCPConfigFile(t *testing.T) {
	path, err := WriteMCPConfigFile("test-del", MCPConfig(9783, "deletetoken"))
	if err != nil {
		t.Fatalf("WriteMCPConfigFile: %v", err)
	}

	DeleteMCPConfigFile("test-del")

	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Errorf("expected file to be deleted, stat err: %v", err)
	}
}

func TestDeleteMCPConfigFile_NoFile(t *testing.T) {
	// Should not panic or return error when file doesn't exist.
	DeleteMCPConfigFile("test-delete-nonexistent-xyz")
}

func TestAgentMCPConfigPath(t *testing.T) {
	path := AgentMCPConfigPath("yuki")
	if path == "" {
		t.Fatal("AgentMCPConfigPath returned empty string")
	}
	if !strings.HasSuffix(path, "/yuki.json") {
		t.Errorf("expected path ending in /yuki.json, got %q", path)
	}
	if !strings.Contains(path, ".ttal/mcps") {
		t.Errorf("expected path under .ttal/mcps, got %q", path)
	}
}
