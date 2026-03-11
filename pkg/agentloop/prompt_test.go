package agentloop

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBuildSystemPrompt_AllFields(t *testing.T) {
	data := PromptData{
		WorkingDir:   "/home/user/project",
		Platform:     "linux",
		Date:         "2026-03-12",
		AllowedPaths: []string{"/home/user/project", "/tmp"},
		Tools: []ToolInfo{
			{Name: "bash", Description: "Execute a bash command."},
			{Name: "glob", Description: "Find files matching a pattern."},
		},
	}

	result, err := BuildSystemPrompt(data)
	require.NoError(t, err)

	assert.Contains(t, result, "/home/user/project")
	assert.Contains(t, result, "linux")
	assert.Contains(t, result, "2026-03-12")
	assert.Contains(t, result, "/tmp")
	assert.Contains(t, result, "bash")
	assert.Contains(t, result, "Execute a bash command.")
	assert.Contains(t, result, "glob")
	assert.Contains(t, result, "Find files matching a pattern.")
	assert.Contains(t, result, "# Available Tools")
}

func TestBuildSystemPrompt_EmptyAllowedPaths_OmitsSection(t *testing.T) {
	data := PromptData{
		WorkingDir:   "/home/user/project",
		Platform:     "darwin",
		Date:         "2026-03-12",
		AllowedPaths: nil,
		Tools:        []ToolInfo{{Name: "bash", Description: "Execute a bash command."}},
	}

	result, err := BuildSystemPrompt(data)
	require.NoError(t, err)

	assert.NotContains(t, result, "# Allowed Paths")
}

func TestBuildSystemPrompt_NoTools_SectionStillPresent(t *testing.T) {
	data := PromptData{
		WorkingDir:   "/home/user/project",
		Platform:     "darwin",
		Date:         "2026-03-12",
		AllowedPaths: nil,
		Tools:        []ToolInfo{},
	}

	result, err := BuildSystemPrompt(data)
	require.NoError(t, err)

	assert.Contains(t, result, "# Available Tools")
	assert.NotEmpty(t, strings.TrimSpace(result))
}

func TestBuildSystemPrompt_ContainsEnvironmentSection(t *testing.T) {
	data := PromptData{
		WorkingDir: "/project",
		Platform:   "linux",
		Date:       "2026-03-12",
	}

	result, err := BuildSystemPrompt(data)
	require.NoError(t, err)

	assert.Contains(t, result, "# Environment")
	assert.Contains(t, result, "/project")
	assert.Contains(t, result, "linux")
	assert.Contains(t, result, "2026-03-12")
}

func TestBuildSystemPrompt_ReturnsNonEmptyString(t *testing.T) {
	data := PromptData{}

	result, err := BuildSystemPrompt(data)
	require.NoError(t, err)

	assert.NotEmpty(t, strings.TrimSpace(result))
}

func TestBuildSystemPrompt_AllowedPathsSection(t *testing.T) {
	data := PromptData{
		AllowedPaths: []string{"/code/repo", "/tmp/scratch"},
	}

	result, err := BuildSystemPrompt(data)
	require.NoError(t, err)

	assert.Contains(t, result, "# Allowed Paths")
	assert.Contains(t, result, "/code/repo")
	assert.Contains(t, result, "/tmp/scratch")
}

// TestSystemPromptComposition tests the append pattern used in cmd/subagent.go:
// basePrompt + "\n\n" + consumerInstructions.
func TestSystemPromptComposition_AppendsConsumerInstructions(t *testing.T) {
	data := PromptData{WorkingDir: "/project", Platform: "linux", Date: "2026-03-12"}

	base, err := BuildSystemPrompt(data)
	require.NoError(t, err)

	consumerInstructions := "You are a code reviewer. Focus on correctness."
	combined := base + "\n\n" + consumerInstructions

	assert.Contains(t, combined, "# Environment")
	assert.Contains(t, combined, consumerInstructions)
	// Consumer instructions come after the default prompt.
	assert.Greater(t, strings.Index(combined, consumerInstructions),
		strings.Index(combined, "# Environment"))
}

func TestSystemPromptComposition_EmptyConsumer_NoAppend(t *testing.T) {
	data := PromptData{WorkingDir: "/project", Platform: "linux", Date: "2026-03-12"}

	base, err := BuildSystemPrompt(data)
	require.NoError(t, err)

	// When --system is empty, no append should happen.
	consumerInstructions := ""
	systemPrompt := base
	if consumerInstructions != "" {
		systemPrompt = base + "\n\n" + consumerInstructions
	}

	assert.Equal(t, base, systemPrompt)
}

// TestBuildSystemPrompt_TemplateSpecialChars documents that text/template does not
// escape WorkingDir or AllowedPaths values — callers should not put template syntax there.
// This test verifies the current contract: values are rendered literally.
func TestBuildSystemPrompt_TemplateSpecialChars(t *testing.T) {
	data := PromptData{
		WorkingDir:   "/home/user/my project",
		AllowedPaths: []string{"/path/with#hash"},
		Tools:        []ToolInfo{{Name: "bash", Description: "desc"}},
	}

	result, err := BuildSystemPrompt(data)
	require.NoError(t, err)

	assert.Contains(t, result, "/home/user/my project")
	assert.Contains(t, result, "/path/with#hash")
}
