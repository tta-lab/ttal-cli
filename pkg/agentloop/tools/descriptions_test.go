package tools

import (
	"context"
	"strings"
	"testing"

	"charm.land/fantasy"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/tta-lab/ttal-cli/pkg/agentloop/sandbox"
)

func TestEmbeddedDescriptions_NonEmpty(t *testing.T) {
	descs := map[string]string{
		"bash":       bashDescription,
		"read":       readDescription,
		"read_md":    readMDDescription,
		"read_url":   readURLDescription,
		"search_web": searchWebDescription,
		"glob":       globDescription,
		"grep":       grepDescription,
	}
	for name, desc := range descs {
		t.Run(name, func(t *testing.T) {
			assert.NotEmpty(t, desc)
		})
	}
}

func TestEmbeddedDescriptions_FirstLineIsOneLiner(t *testing.T) {
	descs := map[string]string{
		"bash":       bashDescription,
		"read":       readDescription,
		"read_md":    readMDDescription,
		"read_url":   readURLDescription,
		"search_web": searchWebDescription,
		"glob":       globDescription,
		"grep":       grepDescription,
	}
	for name, desc := range descs {
		t.Run(name, func(t *testing.T) {
			firstLine := strings.SplitN(desc, "\n", 2)[0]
			assert.NotEmpty(t, firstLine, "first line should not be blank")
			assert.NotContains(t, firstLine, "\n", "first line should be a single line")
		})
	}
}

func TestSchemaDescription_ReturnsFirstLine(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "multi-line",
			input:    "First line.\n\nMore content here.",
			expected: "First line.",
		},
		{
			name:     "single-line",
			input:    "Only one line.",
			expected: "Only one line.",
		},
		{
			name:     "with-surrounding-whitespace",
			input:    "  First line.  \nMore content.",
			expected: "First line.",
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result := schemaDescription(tc.input)
			assert.Equal(t, tc.expected, result)
		})
	}
}

func TestRichToolDescriptions_ReturnsFullContent(t *testing.T) {
	sbx := sandbox.New(sandbox.Options{AllowUnsandboxed: true})
	backend := NewDefuddleCLIBackend()
	allTools := NewDefaultToolSet(sbx, backend, []string{t.TempDir()}, 0)

	richDescs := RichToolDescriptions(allTools)

	require.Len(t, richDescs, len(allTools))

	// Find bash tool and verify it has the full description (more than just the first line).
	found := false
	for _, d := range richDescs {
		if d.Name == "bash" {
			found = true
			firstLine := schemaDescription(bashDescription)
			assert.Greater(t, len(d.Description), len(firstLine),
				"full description should be longer than the schema summary")
			assert.Contains(t, d.Description, firstLine)
		}
	}
	require.True(t, found, "bash tool not found in rich descriptions")
}

func TestRichToolDescriptions_OnlyIncludesProvidedTools(t *testing.T) {
	sbx := sandbox.New(sandbox.Options{AllowUnsandboxed: true})
	backend := NewDefuddleCLIBackend()
	// No allowed paths → only bash, read_url, search_web.
	allTools := NewDefaultToolSet(sbx, backend, nil, 0)

	richDescs := RichToolDescriptions(allTools)

	require.Len(t, richDescs, 3)

	names := make([]string, len(richDescs))
	for i, d := range richDescs {
		names[i] = d.Name
	}
	assert.Contains(t, names, "bash")
	assert.Contains(t, names, "read_url")
	assert.Contains(t, names, "search_web")
	assert.NotContains(t, names, "read")
	assert.NotContains(t, names, "glob")
	assert.NotContains(t, names, "grep")
}

func TestRichToolDescriptions_FallbackToSchemaDescription(t *testing.T) {
	// A tool whose name is not in richDescriptions falls back to its schema description.
	type emptyParams struct{}
	customTool := fantasy.NewAgentTool(
		"custom_unknown_tool",
		"The schema-level description.",
		func(_ context.Context, _ emptyParams, _ fantasy.ToolCall) (fantasy.ToolResponse, error) {
			return fantasy.NewTextResponse(""), nil
		},
	)

	richDescs := RichToolDescriptions([]fantasy.AgentTool{customTool})

	require.Len(t, richDescs, 1)
	assert.Equal(t, "custom_unknown_tool", richDescs[0].Name)
	assert.Equal(t, "The schema-level description.", richDescs[0].Description)
}

func TestToolSchemaDescriptions_MatchFirstLineOfEmbeddedMd(t *testing.T) {
	sbx := sandbox.New(sandbox.Options{AllowUnsandboxed: true})
	backend := NewDefuddleCLIBackend()
	allTools := NewDefaultToolSet(sbx, backend, []string{t.TempDir()}, 0)

	for _, tool := range allTools {
		name := tool.Info().Name
		full, ok := richDescriptions[name]
		if !ok {
			continue // tool has no embedded .md; skip
		}
		t.Run(name, func(t *testing.T) {
			expected := schemaDescription(full)
			assert.Equal(t, expected, tool.Info().Description,
				"schema description should equal first line of embedded .md")
		})
	}
}
