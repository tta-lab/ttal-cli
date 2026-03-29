package sync

import (
	"strings"
	"testing"
)

func TestParseAgentFile(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    AgentFrontmatter
		body    string
		wantErr string
	}{
		{
			name: "full canonical format",
			input: `---
name: task-creator
description: Mechanical taskwarrior task creation

claude-code:
  model: haiku
  tools: ["Bash", "Read"]
---

System prompt body here.
`,
			want: AgentFrontmatter{
				Name:        "task-creator",
				Description: "Mechanical taskwarrior task creation",
				ClaudeCode: map[string]interface{}{
					"model": "haiku",
					"tools": []interface{}{"Bash", "Read"},
				},
			},
			body: "System prompt body here.",
		},
		{
			name: "shared fields only (no runtime blocks)",
			input: `---
name: simple-agent
description: A simple agent
---

Just a body.
`,
			want: AgentFrontmatter{
				Name:        "simple-agent",
				Description: "A simple agent",
			},
			body: "Just a body.",
		},
		{
			name:    "missing name",
			input:   "---\ndescription: no name\n---\nbody\n",
			wantErr: "missing required field: name",
		},
		{
			name:    "missing opening delimiter",
			input:   "no frontmatter here",
			wantErr: "missing opening ---",
		},
		{
			name:    "missing closing delimiter",
			input:   "---\nname: foo\n",
			wantErr: "missing closing ---",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ParseAgentFile(tt.input)

			if tt.wantErr != "" {
				if err == nil {
					t.Fatalf("expected error containing %q, got nil", tt.wantErr)
				}
				if !strings.Contains(err.Error(), tt.wantErr) {
					t.Fatalf("expected error containing %q, got %q", tt.wantErr, err.Error())
				}
				return
			}

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if got.Frontmatter.Name != tt.want.Name {
				t.Errorf("Name = %q, want %q", got.Frontmatter.Name, tt.want.Name)
			}
			if got.Frontmatter.Description != tt.want.Description {
				t.Errorf("Description = %q, want %q", got.Frontmatter.Description, tt.want.Description)
			}
			if got.Body != tt.body {
				t.Errorf("Body = %q, want %q", got.Body, tt.body)
			}

			// Check runtime block presence
			if tt.want.ClaudeCode != nil && got.Frontmatter.ClaudeCode == nil {
				t.Error("expected ClaudeCode to be non-nil")
			}
		})
	}
}

func TestGenerateCCVariant(t *testing.T) {
	agent := &ParsedAgent{
		Frontmatter: AgentFrontmatter{
			Name:        "test-agent",
			Description: "Test description",
			ClaudeCode: map[string]interface{}{
				"model": "haiku",
			},
		},
		Body: "Prompt body.\n",
	}

	result, err := GenerateCCVariant(agent)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !strings.Contains(result, ManagedMarkerField) {
		t.Error("CC variant should contain managed marker field")
	}
	if !strings.Contains(result, "name: test-agent") {
		t.Error("CC variant should contain name")
	}
	if !strings.Contains(result, "model: haiku") {
		t.Error("CC variant should contain CC model")
	}
	if !strings.Contains(result, "Prompt body.") {
		t.Error("CC variant should contain body")
	}
}

func TestGenerateVariantNoRuntimeBlock(t *testing.T) {
	agent := &ParsedAgent{
		Frontmatter: AgentFrontmatter{
			Name:        "minimal",
			Description: "No runtime blocks",
		},
		Body: "Just a body.\n",
	}

	cc, err := GenerateCCVariant(agent)
	if err != nil {
		t.Fatalf("CC variant: %v", err)
	}
	if !strings.Contains(cc, "name: minimal") {
		t.Error("CC variant should still contain shared name")
	}
}

func TestGenerateCCVariantIncludesColor(t *testing.T) {
	agent := &ParsedAgent{
		Frontmatter: AgentFrontmatter{
			Name:  "coder",
			Color: "green",
			ClaudeCode: map[string]interface{}{
				"model": "sonnet",
			},
		},
		Body: "Prompt body.\n",
	}

	result, err := GenerateCCVariant(agent)
	if err != nil {
		t.Fatalf("GenerateCCVariant: %v", err)
	}
	if !strings.Contains(result, "color: green") {
		t.Errorf("CC variant should contain color field, got:\n%s", result)
	}
}

func TestGenerateCCVariantOmitsEmptyColor(t *testing.T) {
	agent := &ParsedAgent{
		Frontmatter: AgentFrontmatter{
			Name:  "no-color",
			Color: "",
		},
		Body: "Body.\n",
	}

	result, err := GenerateCCVariant(agent)
	if err != nil {
		t.Fatalf("GenerateCCVariant: %v", err)
	}
	if strings.Contains(result, "color:") {
		t.Errorf("CC variant should not contain color field when empty, got:\n%s", result)
	}
}
