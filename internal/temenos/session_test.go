package temenos

import (
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
