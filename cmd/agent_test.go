package cmd

import (
	"context"
	"testing"

	"github.com/guion-opensource/ttal-cli/ent/agent"
	"github.com/guion-opensource/ttal-cli/ent/tag"
	"github.com/guion-opensource/ttal-cli/internal/db"
)

func setupAgentTest(t *testing.T) {
	t.Helper()
	database = db.NewTestDB(t)
}

func TestParseModifyArgs(t *testing.T) {
	tests := []struct {
		name          string
		args          []string
		wantAddTags   []string
		wantRemoveTags []string
		wantFields    map[string]string
	}{
		{
			name:          "add tags only",
			args:          []string{"+backend", "+api"},
			wantAddTags:   []string{"backend", "api"},
			wantRemoveTags: []string{},
			wantFields:    map[string]string{},
		},
		{
			name:          "remove tags only",
			args:          []string{"-legacy", "-old"},
			wantAddTags:   []string{},
			wantRemoveTags: []string{"legacy", "old"},
			wantFields:    map[string]string{},
		},
		{
			name:          "field updates only",
			args:          []string{"path:/new/path", "name:test"},
			wantAddTags:   []string{},
			wantRemoveTags: []string{},
			wantFields:    map[string]string{"path": "/new/path", "name": "test"},
		},
		{
			name:          "mixed operations",
			args:          []string{"+backend", "-legacy", "path:/new/path", "+api"},
			wantAddTags:   []string{"backend", "api"},
			wantRemoveTags: []string{"legacy"},
			wantFields:    map[string]string{"path": "/new/path"},
		},
		{
			name:          "field with spaces",
			args:          []string{"name:My Project Name"},
			wantAddTags:   []string{},
			wantRemoveTags: []string{},
			wantFields:    map[string]string{"name": "My Project Name"},
		},
		{
			name:          "empty args",
			args:          []string{},
			wantAddTags:   []string{},
			wantRemoveTags: []string{},
			wantFields:    map[string]string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotAddTags, gotRemoveTags, gotFields := parseModifyArgs(tt.args)

			// Check add tags
			if len(gotAddTags) != len(tt.wantAddTags) {
				t.Errorf("parseModifyArgs() addTags length = %v, want %v", len(gotAddTags), len(tt.wantAddTags))
			}
			for i, tag := range tt.wantAddTags {
				if i >= len(gotAddTags) || gotAddTags[i] != tag {
					t.Errorf("parseModifyArgs() addTags[%d] = %v, want %v", i, gotAddTags[i], tag)
				}
			}

			// Check remove tags
			if len(gotRemoveTags) != len(tt.wantRemoveTags) {
				t.Errorf("parseModifyArgs() removeTags length = %v, want %v", len(gotRemoveTags), len(tt.wantRemoveTags))
			}
			for i, tag := range tt.wantRemoveTags {
				if i >= len(gotRemoveTags) || gotRemoveTags[i] != tag {
					t.Errorf("parseModifyArgs() removeTags[%d] = %v, want %v", i, gotRemoveTags[i], tag)
				}
			}

			// Check fields
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

func TestAgentModifyPath(t *testing.T) {
	setupAgentTest(t)
	ctx := context.Background()

	// Create agent
	ag, err := database.Agent.Create().
		SetName("test-agent").
		SetPath("/old/path").
		Save(ctx)
	if err != nil {
		t.Fatalf("failed to create agent: %v", err)
	}

	// Modify path
	_, err = ag.Update().
		SetPath("/new/path").
		Save(ctx)
	if err != nil {
		t.Fatalf("failed to update agent path: %v", err)
	}

	// Verify
	updated, err := database.Agent.Query().
		Where(agent.Name("test-agent")).
		Only(ctx)
	if err != nil {
		t.Fatalf("failed to query agent: %v", err)
	}

	if updated.Path != "/new/path" {
		t.Errorf("agent path = %v, want %v", updated.Path, "/new/path")
	}
}

func TestAgentModifyTags(t *testing.T) {
	setupAgentTest(t)
	ctx := context.Background()

	// Create tags
	tag1, err := database.Tag.Create().SetName("backend").Save(ctx)
	if err != nil {
		t.Fatalf("failed to create tag1: %v", err)
	}
	tag2, err := database.Tag.Create().SetName("frontend").Save(ctx)
	if err != nil {
		t.Fatalf("failed to create tag2: %v", err)
	}

	// Create agent with tag1
	ag, err := database.Agent.Create().
		SetName("test-agent").
		AddTags(tag1).
		Save(ctx)
	if err != nil {
		t.Fatalf("failed to create agent: %v", err)
	}

	// Add tag2, keep tag1
	_, err = ag.Update().
		AddTags(tag2).
		Save(ctx)
	if err != nil {
		t.Fatalf("failed to add tag: %v", err)
	}

	// Verify agent has both tags
	updated, err := database.Agent.Query().
		Where(agent.Name("test-agent")).
		WithTags().
		Only(ctx)
	if err != nil {
		t.Fatalf("failed to query agent: %v", err)
	}

	if len(updated.Edges.Tags) != 2 {
		t.Errorf("agent has %d tags, want 2", len(updated.Edges.Tags))
	}

	// Remove tag1
	_, err = updated.Update().
		RemoveTags(tag1).
		Save(ctx)
	if err != nil {
		t.Fatalf("failed to remove tag: %v", err)
	}

	// Verify agent only has tag2
	updated, err = database.Agent.Query().
		Where(agent.Name("test-agent")).
		WithTags().
		Only(ctx)
	if err != nil {
		t.Fatalf("failed to query agent: %v", err)
	}

	if len(updated.Edges.Tags) != 1 {
		t.Errorf("agent has %d tags, want 1", len(updated.Edges.Tags))
	}
	if updated.Edges.Tags[0].Name != "frontend" {
		t.Errorf("agent tag = %v, want frontend", updated.Edges.Tags[0].Name)
	}
}

func TestAgentModifyCombined(t *testing.T) {
	setupAgentTest(t)
	ctx := context.Background()

	// Create tags
	oldTag, err := database.Tag.Create().SetName("old").Save(ctx)
	if err != nil {
		t.Fatalf("failed to create old tag: %v", err)
	}
	newTag, err := database.Tag.Create().SetName("new").Save(ctx)
	if err != nil {
		t.Fatalf("failed to create new tag: %v", err)
	}

	// Create agent with old tag and old path
	ag, err := database.Agent.Create().
		SetName("test-agent").
		SetPath("/old/path").
		AddTags(oldTag).
		Save(ctx)
	if err != nil {
		t.Fatalf("failed to create agent: %v", err)
	}

	// Modify path and tags in one operation
	_, err = ag.Update().
		SetPath("/new/path").
		AddTags(newTag).
		RemoveTags(oldTag).
		Save(ctx)
	if err != nil {
		t.Fatalf("failed to update agent: %v", err)
	}

	// Verify
	updated, err := database.Agent.Query().
		Where(agent.Name("test-agent")).
		WithTags().
		Only(ctx)
	if err != nil {
		t.Fatalf("failed to query agent: %v", err)
	}

	if updated.Path != "/new/path" {
		t.Errorf("agent path = %v, want /new/path", updated.Path)
	}
	if len(updated.Edges.Tags) != 1 {
		t.Errorf("agent has %d tags, want 1", len(updated.Edges.Tags))
	}
	if updated.Edges.Tags[0].Name != "new" {
		t.Errorf("agent tag = %v, want new", updated.Edges.Tags[0].Name)
	}
}

func TestAgentQueryWithTags(t *testing.T) {
	setupAgentTest(t)
	ctx := context.Background()

	// Create tags
	backendTag, err := database.Tag.Create().SetName("backend").Save(ctx)
	if err != nil {
		t.Fatalf("failed to create backend tag: %v", err)
	}
	frontendTag, err := database.Tag.Create().SetName("frontend").Save(ctx)
	if err != nil {
		t.Fatalf("failed to create frontend tag: %v", err)
	}

	// Create agents
	_, err = database.Agent.Create().
		SetName("agent1").
		AddTags(backendTag).
		Save(ctx)
	if err != nil {
		t.Fatalf("failed to create agent1: %v", err)
	}

	_, err = database.Agent.Create().
		SetName("agent2").
		AddTags(frontendTag).
		Save(ctx)
	if err != nil {
		t.Fatalf("failed to create agent2: %v", err)
	}

	_, err = database.Agent.Create().
		SetName("agent3").
		AddTags(backendTag, frontendTag).
		Save(ctx)
	if err != nil {
		t.Fatalf("failed to create agent3: %v", err)
	}

	// Query agents with backend tag
	backendAgents, err := database.Agent.Query().
		Where(agent.HasTagsWith(tag.Name("backend"))).
		All(ctx)
	if err != nil {
		t.Fatalf("failed to query backend agents: %v", err)
	}

	if len(backendAgents) != 2 {
		t.Errorf("found %d backend agents, want 2", len(backendAgents))
	}

	// Query agents with frontend tag
	frontendAgents, err := database.Agent.Query().
		Where(agent.HasTagsWith(tag.Name("frontend"))).
		All(ctx)
	if err != nil {
		t.Fatalf("failed to query frontend agents: %v", err)
	}

	if len(frontendAgents) != 2 {
		t.Errorf("found %d frontend agents, want 2", len(frontendAgents))
	}
}
