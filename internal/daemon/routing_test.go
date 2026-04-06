package daemon

import (
	"strings"
	"testing"

	"github.com/tta-lab/ttal-cli/internal/config"
)

func TestParseWorkerAddress(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		wantJob  string
		wantName string
		wantOK   bool
	}{
		{
			name:     "valid job_id:agent_name",
			input:    "abc12345:coder",
			wantJob:  "abc12345",
			wantName: "coder",
			wantOK:   true,
		},
		{
			name:     "valid mixed case",
			input:    "AABBCCDD:reviewer",
			wantJob:  "AABBCCDD",
			wantName: "reviewer",
			wantOK:   true,
		},
		{
			name:     "exact 8 chars",
			input:    "aabbccdd:coder",
			wantJob:  "aabbccdd",
			wantName: "coder",
			wantOK:   true,
		},
		{
			name:     "multi-colon (agent_name contains colon)",
			input:    "abc12345:team:coder",
			wantJob:  "abc12345",
			wantName: "team:coder",
			wantOK:   true,
		},
		{
			name:   "bare hex is invalid",
			input:  "abc12345",
			wantOK: false,
		},
		{
			name:   "no colon",
			input:  "kestrel",
			wantOK: false,
		},
		{
			name:   "short hex",
			input:  "abc:coder",
			wantOK: false,
		},
		{
			name:   "non-hex prefix",
			input:  "zzzzzzzz:coder",
			wantOK: false,
		},
		{
			name:   "empty agent name",
			input:  "abc12345:",
			wantOK: false,
		},
		{
			name:   "empty string",
			input:  "",
			wantOK: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			jobID, agentName, ok := parseWorkerAddress(tt.input)
			if ok != tt.wantOK {
				t.Errorf("parseWorkerAddress(%q) ok = %v, want %v", tt.input, ok, tt.wantOK)
				return
			}
			if tt.wantOK {
				if jobID != tt.wantJob {
					t.Errorf("parseWorkerAddress(%q) jobID = %q, want %q", tt.input, jobID, tt.wantJob)
				}
				if agentName != tt.wantName {
					t.Errorf("parseWorkerAddress(%q) agentName = %q, want %q", tt.input, agentName, tt.wantName)
				}
			}
		})
	}
}

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
			// Non-hex string: fails parseWorkerAddress's character check.
			name:    "non-hex from is rejected",
			from:    "not-a-worker",
			wantErr: "unknown agent or worker",
		},
		{
			// Short hex (< 8 chars): fails parseWorkerAddress's length check.
			name:    "short hex from is rejected",
			from:    "abc123",
			wantErr: "unknown agent or worker",
		},
		{
			// Non-hex 8-char string: fails parseWorkerAddress's character loop.
			name:    "non-hex 8-char from is rejected",
			from:    "gggggggg",
			wantErr: "unknown agent or worker",
		},
		{
			// Compound format but no matching tmux session — resolveWorker errors.
			name:    "compound with no tmux server is rejected",
			from:    "aabbccdd:coder",
			wantErr: "unknown agent or worker",
		},
		{
			// Bare hex (old format): resolveWorker would succeed but we now reject it.
			name:    "bare hex from is rejected",
			from:    "aabbccdd",
			wantErr: "bare worker UUID not supported",
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

// TestHandleToRejectsBareHex verifies that handleTo rejects bare hex UUIDs
// with a helpful error message.
func TestHandleToRejectsBareHex(t *testing.T) {
	mcfg := &config.DaemonConfig{}

	req := SendRequest{
		To:      "aabbccdd",
		Message: "test",
	}
	err := handleTo(mcfg, nil, nil, nil, req)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "bare worker UUID not supported") {
		t.Errorf("error = %q, want substring %q", err.Error(), "bare worker UUID not supported")
	}
	if !strings.Contains(err.Error(), "job_id:agent_name") {
		t.Errorf("error = %q, want substring %q", err.Error(), "job_id:agent_name")
	}
}
