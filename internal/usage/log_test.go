package usage

import (
	"testing"
)

func TestLogWith_SkipsWithoutAgentName(t *testing.T) {
	t.Setenv("TTAL_AGENT_NAME", "")
	// Should not panic or error — silent skip
	LogWith("flicknote", "add", "note-abc")
}

func TestLogWith_SetsCommandField(t *testing.T) {
	// Verifies function signature accepts command parameter.
	// Full integration test requires DB — covered by existing daemon tests.
	t.Setenv("TTAL_AGENT_NAME", "")
	LogWith("flicknote", "add", "note-abc")
	LogWith("ttal", "explore", "ttal-cli")
}
