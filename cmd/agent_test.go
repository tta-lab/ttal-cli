package cmd

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/tta-lab/ttal-cli/internal/agentfs"
)

func TestParseModifyArgs(t *testing.T) {
	tests := []struct {
		name       string
		args       []string
		wantFields map[string]string
		wantErr    bool
	}{
		{
			name:       "field updates only",
			args:       []string{"voice:af_heart", "emoji:🐱"},
			wantFields: map[string]string{"voice": "af_heart", "emoji": "🐱"},
		},
		{
			name:       "field with spaces",
			args:       []string{"name:My Project Name"},
			wantFields: map[string]string{"name": "My Project Name"},
		},
		{
			name:       "empty args",
			args:       []string{},
			wantFields: map[string]string{},
		},
		{
			name:    "add tags returns error",
			args:    []string{"+backend", "+api"},
			wantErr: true,
		},
		{
			name:    "remove tags returns error",
			args:    []string{"-legacy"},
			wantErr: true,
		},
		{
			name:    "mixed tags and fields returns error",
			args:    []string{"+backend", "voice:af_heart"},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotFields, err := parseModifyArgs(tt.args)

			if tt.wantErr {
				if err == nil {
					t.Errorf("parseModifyArgs() expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Errorf("parseModifyArgs() unexpected error: %v", err)
				return
			}

			if len(gotFields) != len(tt.wantFields) {
				t.Errorf("parseModifyArgs() fields length = %v, want %v", len(gotFields), len(tt.wantFields))
			}
			for key, val := range tt.wantFields {
				if gotFields[key] != val {
					t.Errorf("parseModifyArgs() fields[%s] = %v, want %v", key, gotFields[key], val)
				}
			}
		})
	}
}

func TestAgentfsSetField(t *testing.T) {
	dir := t.TempDir()
	content := []byte("---\nvoice: af_heart\n---\n# test-agent\n")
	if err := os.WriteFile(filepath.Join(dir, "test-agent.md"), content, 0o644); err != nil {
		t.Fatal(err)
	}

	// Update voice
	if err := agentfs.SetField(dir, "test-agent", "voice", "af_sky"); err != nil {
		t.Fatalf("SetField: %v", err)
	}

	// Verify
	ag, err := agentfs.Get(dir, "test-agent")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if ag.Voice != "af_sky" {
		t.Errorf("agent voice = %v, want af_sky", ag.Voice)
	}
}
