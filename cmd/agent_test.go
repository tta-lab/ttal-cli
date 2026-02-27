package cmd

import (
	"context"
	"testing"

	"codeberg.org/clawteam/ttal-cli/ent/agent"
	"codeberg.org/clawteam/ttal-cli/internal/db"
)

func setupAgentTest(t *testing.T) {
	t.Helper()
	database = db.NewTestDB(t)
}

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

func TestAgentModifyVoice(t *testing.T) {
	setupAgentTest(t)
	ctx := context.Background()

	ag, err := database.Agent.Create().
		SetName("test-agent").
		SetVoice("af_heart").
		Save(ctx)
	if err != nil {
		t.Fatalf("failed to create agent: %v", err)
	}

	_, err = ag.Update().
		SetVoice("af_sky").
		Save(ctx)
	if err != nil {
		t.Fatalf("failed to update agent voice: %v", err)
	}

	updated, err := database.Agent.Query().
		Where(agent.Name("test-agent")).
		Only(ctx)
	if err != nil {
		t.Fatalf("failed to query agent: %v", err)
	}

	if updated.Voice != "af_sky" {
		t.Errorf("agent voice = %v, want %v", updated.Voice, "af_sky")
	}
}
