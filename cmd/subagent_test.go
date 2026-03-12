package cmd

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/tta-lab/ttal-cli/pkg/agentloop/sandbox"
	"github.com/tta-lab/ttal-cli/pkg/agentloop/tools"
)

func TestFilterTools_EmptyNames_ReturnsAll(t *testing.T) {
	sbx := &sandbox.Sandbox{AllowUnsandboxed: true}
	backend := tools.NewDefuddleCLIBackend()
	allTools := tools.NewDefaultToolSet(sbx, backend, nil, 0)

	selected, err := filterTools(allTools, nil)
	require.NoError(t, err)
	assert.Equal(t, allTools, selected)
}

func TestFilterTools_ValidNames_ReturnsSubset(t *testing.T) {
	sbx := &sandbox.Sandbox{AllowUnsandboxed: true}
	backend := tools.NewDefuddleCLIBackend()
	allTools := tools.NewDefaultToolSet(sbx, backend, nil, 0)

	selected, err := filterTools(allTools, []string{"bash"})
	require.NoError(t, err)
	require.Len(t, selected, 1)
	assert.Equal(t, "bash", selected[0].Info().Name)
}

func TestFilterTools_UnknownName_ReturnsError(t *testing.T) {
	sbx := &sandbox.Sandbox{AllowUnsandboxed: true}
	backend := tools.NewDefuddleCLIBackend()
	allTools := tools.NewDefaultToolSet(sbx, backend, nil, 0)

	_, err := filterTools(allTools, []string{"nonexistent_tool"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "nonexistent_tool")
	assert.Contains(t, err.Error(), "available")
}

func TestFilterTools_MixedNames_ErrorOnFirst_Unknown(t *testing.T) {
	sbx := &sandbox.Sandbox{AllowUnsandboxed: true}
	backend := tools.NewDefuddleCLIBackend()
	allTools := tools.NewDefaultToolSet(sbx, backend, nil, 0)

	_, err := filterTools(allTools, []string{"bash", "unknown_tool"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "unknown_tool")
}
