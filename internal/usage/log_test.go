package usage

import (
	"testing"
)

func TestLogWith_SkipsWithoutAgentName(t *testing.T) {
	t.Setenv("TTAL_AGENT_NAME", "")
	// Should not panic or error — silent skip
	LogWith("flicknote", "add", "note-abc")
}

func TestLogWith_AcceptsArbitraryCommand(t *testing.T) {
	// Verifies the function signature accepts any command string (not just "ttal").
	// Full DB write is skipped because TTAL_AGENT_NAME is unset — this confirms
	// the silent-skip path handles multiple callers without panicking.
	t.Setenv("TTAL_AGENT_NAME", "")
	LogWith("flicknote", "add", "note-abc")
	LogWith("flicknote", "modify", "note-xyz")
}
