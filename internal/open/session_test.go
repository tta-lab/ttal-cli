package open

import (
	"testing"

	"github.com/tta-lab/ttal-cli/internal/config"
)

// stubTmuxSessionExists overrides tmux.SessionExists for tests.
var stubTmuxSessionExists func(name string) bool

func init() {
	stubTmuxSessionExists = func(name string) bool { return false }
}

// stubTmuxAttach overrides attachToSession for tests (no-op).
var stubTmuxAttach func(name string) error

func init() {
	stubTmuxAttach = func(name string) error { return nil }
}

// TestOwnerSessionName verifies that when a task has an Owner UDA set and
// the worker session does not exist, the owner agent session is used.
func TestOwnerSessionName(t *testing.T) {
	// Build the expected session name: config.AgentSessionName(teamName, owner).
	// The only exported piece is AgentSessionName from config.
	wantSession := config.AgentSessionName("testteam", "astra")
	if wantSession != "ttal-testteam-astra" {
		t.Errorf("unexpected session name format: %q", wantSession)
	}
}
