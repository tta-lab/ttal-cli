package cmd

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/tta-lab/ttal-cli/internal/ask"
)

func TestBuildSubagentProvider_Anthropic(t *testing.T) {
	t.Setenv("ANTHROPIC_API_KEY", "test-key")
	p, modelID, err := ask.BuildProvider("claude-sonnet-4-6")
	require.NoError(t, err)
	assert.NotNil(t, p)
	assert.Equal(t, "claude-sonnet-4-6", modelID)
}

func TestBuildSubagentProvider_MissingKey(t *testing.T) {
	t.Setenv("ANTHROPIC_API_KEY", "")
	_, _, err := ask.BuildProvider("claude-sonnet-4-6")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "ANTHROPIC_API_KEY")
}
