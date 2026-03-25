package daemon

import (
	"strings"
	"testing"

	"github.com/tta-lab/ttal-cli/internal/config"
)

// TestHandleAgentToAgentUnknownSender verifies the negative-path behaviour for
// the From field in handleAgentToAgent after the worker-hex-ID fix.
//
// Success-path verification (a real worker session sending an alert) requires a
// live tmux server and cannot run in CI. These tests cover the regression:
// before the fix, any unknown From returned "unknown agent: X". After the fix,
// only truly unresolvable senders error — and the error message says
// "unknown agent or worker".
func TestHandleAgentToAgentUnknownSender(t *testing.T) {
	mcfg := &config.DaemonConfig{}

	tests := []struct {
		name    string
		from    string
		wantErr string
	}{
		{
			// Non-hex string: fails resolveWorker's character check before any tmux call.
			name:    "non-hex from is rejected",
			from:    "not-a-worker",
			wantErr: "unknown agent or worker",
		},
		{
			// Short hex (< 8 chars): fails resolveWorker's length check before any tmux call.
			name:    "short hex from is rejected",
			from:    "abc123",
			wantErr: "unknown agent or worker",
		},
		{
			// Non-hex 8-char string: fails resolveWorker's character loop before any tmux call.
			name:    "non-hex 8-char from is rejected",
			from:    "gggggggg",
			wantErr: "unknown agent or worker",
		},
		{
			// Valid hex format but no matching tmux session — hits ListSessions which errors
			// when no tmux server is running (expected in CI).
			name:    "valid hex from with no tmux server is rejected",
			from:    "aabbccdd",
			wantErr: "unknown agent or worker",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := SendRequest{
				From:    tt.from,
				To:      "kestrel",
				Message: "test",
			}
			err := handleAgentToAgent(mcfg, nil, nil, nil, req)
			if err == nil {
				t.Fatal("expected error, got nil")
			}
			if !strings.Contains(err.Error(), tt.wantErr) {
				t.Errorf("error = %q, want substring %q", err.Error(), tt.wantErr)
			}
		})
	}
}
