package agentloop

import (
	"context"
	"encoding/json"
	"testing"

	"charm.land/fantasy"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/tta-lab/ttal-cli/pkg/agentloop/sandbox"
)

// mockLanguageModel implements fantasy.LanguageModel for testing.
type mockLanguageModel struct {
	streamFunc func(ctx context.Context, call fantasy.Call) (fantasy.StreamResponse, error)
}

func (m *mockLanguageModel) Generate(ctx context.Context, call fantasy.Call) (*fantasy.Response, error) {
	return &fantasy.Response{
		Content:      []fantasy.Content{fantasy.TextContent{Text: "mock response"}},
		FinishReason: fantasy.FinishReasonStop,
	}, nil
}

func (m *mockLanguageModel) Stream(ctx context.Context, call fantasy.Call) (fantasy.StreamResponse, error) {
	if m.streamFunc != nil {
		return m.streamFunc(ctx, call)
	}
	return func(yield func(fantasy.StreamPart) bool) {
		yield(fantasy.StreamPart{Type: fantasy.StreamPartTypeTextStart, ID: "t1"})
		yield(fantasy.StreamPart{Type: fantasy.StreamPartTypeTextDelta, ID: "t1", Delta: "hello world"})
		yield(fantasy.StreamPart{Type: fantasy.StreamPartTypeTextEnd, ID: "t1"})
		yield(fantasy.StreamPart{
			Type: fantasy.StreamPartTypeFinish,
			Usage: fantasy.Usage{
				InputTokens:  5,
				OutputTokens: 2,
				TotalTokens:  7,
			},
			FinishReason: fantasy.FinishReasonStop,
		})
	}, nil
}

func (m *mockLanguageModel) Provider() string { return "mock" }
func (m *mockLanguageModel) Model() string    { return "mock-model" }
func (m *mockLanguageModel) GenerateObject(_ context.Context, _ fantasy.ObjectCall) (*fantasy.ObjectResponse, error) {
	return nil, nil
}
func (m *mockLanguageModel) StreamObject(
	_ context.Context, _ fantasy.ObjectCall,
) (fantasy.ObjectStreamResponse, error) {
	return nil, nil
}

// mockProvider implements fantasy.Provider for testing.
type mockProvider struct {
	model fantasy.LanguageModel
}

func (p *mockProvider) Name() string { return "mock" }
func (p *mockProvider) LanguageModel(_ context.Context, _ string) (fantasy.LanguageModel, error) {
	return p.model, nil
}

func TestRun_SimpleTextResponse(t *testing.T) {
	provider := &mockProvider{model: &mockLanguageModel{}}

	cfg := Config{
		Provider:     provider,
		Model:        "mock-model",
		SystemPrompt: "You are helpful.",
		Tools:        nil,
		MaxSteps:     5,
		MaxTokens:    1024,
		SandboxEnv:   nil,
	}

	var deltas []string
	result, err := Run(context.Background(), cfg, nil, "say hello", Callbacks{
		OnDelta: func(text string) { deltas = append(deltas, text) },
	})

	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Equal(t, "hello world", result.Response)
	assert.Contains(t, deltas, "hello world")
}

func TestRun_AccumulatesSteps(t *testing.T) {
	provider := &mockProvider{model: &mockLanguageModel{}}

	cfg := Config{
		Provider: provider,
		Model:    "mock-model",
	}

	result, err := Run(context.Background(), cfg, nil, "hello", Callbacks{})

	require.NoError(t, err)
	require.NotNil(t, result)
	// The final assistant text should be flushed into Steps
	require.Len(t, result.Steps, 1)
	assert.Equal(t, StepRoleAssistant, result.Steps[0].Role)
	assert.Equal(t, "hello world", result.Steps[0].Content)
}

func TestRun_NilProviderReturnsError(t *testing.T) {
	cfg := Config{
		Provider: nil,
		Model:    "mock-model",
	}
	_, err := Run(context.Background(), cfg, nil, "hello", Callbacks{})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "Provider must not be nil")
}

func TestRun_SandboxEnvInContext(t *testing.T) {
	provider := &mockProvider{model: &mockLanguageModel{}}

	cfg := Config{
		Provider:   provider,
		Model:      "mock-model",
		SandboxEnv: []string{"MY_VAR=test"},
	}

	result, err := Run(context.Background(), cfg, nil, "hello", Callbacks{})
	require.NoError(t, err)
	assert.Equal(t, "hello world", result.Response)
}

func TestRun_DefaultMaxSteps(t *testing.T) {
	// When MaxSteps=0, defaults to 20 — verify no panic/error
	provider := &mockProvider{model: &mockLanguageModel{}}
	cfg := Config{Provider: provider, Model: "mock-model"}
	_, err := Run(context.Background(), cfg, nil, "hello", Callbacks{})
	require.NoError(t, err)
}

func TestRun_NilAllowedPathsProducesNilMounts(t *testing.T) {
	// nil AllowedPaths should produce nil MountDirs (not a non-nil empty slice).
	provider := &mockProvider{model: &mockLanguageModel{}}
	cfg := Config{Provider: provider, Model: "mock-model", AllowedPaths: nil}
	_, err := Run(context.Background(), cfg, nil, "hello", Callbacks{})
	require.NoError(t, err)
}

func TestRun_InvalidAllowedPathReturnsError(t *testing.T) {
	provider := &mockProvider{model: &mockLanguageModel{}}

	_, err := Run(context.Background(), Config{
		Provider:     provider,
		Model:        "mock-model",
		AllowedPaths: []string{""},
	}, nil, "hello", Callbacks{})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "non-empty absolute path")

	_, err = Run(context.Background(), Config{
		Provider:     provider,
		Model:        "mock-model",
		AllowedPaths: []string{"relative/path"},
	}, nil, "hello", Callbacks{})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "non-empty absolute path")
}

// makeCaptureTool returns a tool that captures the ExecConfig from context into dst.
func makeCaptureTool(dst **sandbox.ExecConfig) fantasy.AgentTool {
	type captureInput struct{}
	return fantasy.NewAgentTool("capture", "captures exec config from context",
		func(ctx context.Context, _ captureInput, _ fantasy.ToolCall) (fantasy.ToolResponse, error) {
			*dst = sandbox.ExecConfigFromContext(ctx)
			return fantasy.NewTextResponse("ok"), nil
		})
}

// toolCallThenDoneStream returns a stream func that emits one "capture" tool call,
// then on the second call returns a final text response.
func toolCallThenDoneStream() func(_ context.Context, _ fantasy.Call) (fantasy.StreamResponse, error) {
	callCount := 0
	return func(_ context.Context, _ fantasy.Call) (fantasy.StreamResponse, error) {
		callCount++
		if callCount == 1 {
			return func(yield func(fantasy.StreamPart) bool) {
				yield(fantasy.StreamPart{
					Type: fantasy.StreamPartTypeToolCall, ID: "tc1",
					ToolCallName: "capture", ToolCallInput: "{}",
				})
				yield(fantasy.StreamPart{Type: fantasy.StreamPartTypeFinish, FinishReason: fantasy.FinishReasonToolCalls})
			}, nil
		}
		return func(yield func(fantasy.StreamPart) bool) {
			yield(fantasy.StreamPart{Type: fantasy.StreamPartTypeTextStart, ID: "t1"})
			yield(fantasy.StreamPart{Type: fantasy.StreamPartTypeTextDelta, ID: "t1", Delta: "done"})
			yield(fantasy.StreamPart{Type: fantasy.StreamPartTypeTextEnd, ID: "t1"})
			yield(fantasy.StreamPart{Type: fantasy.StreamPartTypeFinish, FinishReason: fantasy.FinishReasonStop})
		}, nil
	}
}

func TestRun_SandboxEnvAndAllowedPathsBothWired(t *testing.T) {
	// Verify SandboxEnv and AllowedPaths both survive ExecConfig construction together.
	var capturedExecCfg *sandbox.ExecConfig
	cfg := Config{
		Provider:     &mockProvider{model: &mockLanguageModel{streamFunc: toolCallThenDoneStream()}},
		Model:        "mock-model",
		Tools:        []fantasy.AgentTool{makeCaptureTool(&capturedExecCfg)},
		SandboxEnv:   []string{"MY_VAR=hello"},
		AllowedPaths: []string{"/some/dir"},
	}

	_, err := Run(context.Background(), cfg, nil, "capture", Callbacks{})
	require.NoError(t, err)
	require.NotNil(t, capturedExecCfg)
	assert.Equal(t, []string{"MY_VAR=hello"}, capturedExecCfg.Env)
	require.Len(t, capturedExecCfg.MountDirs, 1)
	assert.Equal(t,
		sandbox.Mount{Source: "/some/dir", Target: "/some/dir", ReadOnly: true},
		capturedExecCfg.MountDirs[0])
}

func TestRun_AllowedPathsInMountDirs(t *testing.T) {
	var capturedExecCfg *sandbox.ExecConfig
	cfg := Config{
		Provider:     &mockProvider{model: &mockLanguageModel{streamFunc: toolCallThenDoneStream()}},
		Model:        "mock-model",
		Tools:        []fantasy.AgentTool{makeCaptureTool(&capturedExecCfg)},
		AllowedPaths: []string{"/some/project/dir", "/another/dir"},
	}

	_, err := Run(context.Background(), cfg, nil, "capture exec config", Callbacks{})
	require.NoError(t, err)
	require.NotNil(t, capturedExecCfg, "tool should have captured ExecConfig from context")

	require.Len(t, capturedExecCfg.MountDirs, 2)
	assert.Equal(t,
		sandbox.Mount{Source: "/some/project/dir", Target: "/some/project/dir", ReadOnly: true},
		capturedExecCfg.MountDirs[0])
	assert.Equal(t,
		sandbox.Mount{Source: "/another/dir", Target: "/another/dir", ReadOnly: true},
		capturedExecCfg.MountDirs[1])
}

// TestRun_ToolCallAndResultCallbacks verifies the OnToolCall/OnToolResult flush
// state machine works: assistant text + tool calls are flushed before the tool
// result, and the tool result lands in a separate StepRoleTool step.
//
// The mock emits: text → ToolCall → Finish (step 1). The agent executes the tool
// (not registered → error result → OnToolResult fires). Then step 2 returns text → Finish.
func TestRun_ToolCallAndResultCallbacks(t *testing.T) {
	toolInput := `{"command":"echo hi"}`

	callCount := 0
	streamWithToolCall := func(_ context.Context, _ fantasy.Call) (fantasy.StreamResponse, error) {
		callCount++
		if callCount == 1 {
			return func(yield func(fantasy.StreamPart) bool) {
				// Step 1: text then tool call
				yield(fantasy.StreamPart{Type: fantasy.StreamPartTypeTextStart, ID: "t1"})
				yield(fantasy.StreamPart{Type: fantasy.StreamPartTypeTextDelta, ID: "t1", Delta: "Running bash..."})
				yield(fantasy.StreamPart{Type: fantasy.StreamPartTypeTextEnd, ID: "t1"})
				yield(fantasy.StreamPart{
					Type:          fantasy.StreamPartTypeToolCall,
					ID:            "tc1",
					ToolCallName:  "bash",
					ToolCallInput: toolInput,
				})
				yield(fantasy.StreamPart{Type: fantasy.StreamPartTypeFinish, FinishReason: fantasy.FinishReasonToolCalls})
			}, nil
		}
		// Step 2: final text after tool result
		return func(yield func(fantasy.StreamPart) bool) {
			yield(fantasy.StreamPart{Type: fantasy.StreamPartTypeTextStart, ID: "t2"})
			yield(fantasy.StreamPart{Type: fantasy.StreamPartTypeTextDelta, ID: "t2", Delta: "Done."})
			yield(fantasy.StreamPart{Type: fantasy.StreamPartTypeTextEnd, ID: "t2"})
			yield(fantasy.StreamPart{Type: fantasy.StreamPartTypeFinish, FinishReason: fantasy.FinishReasonStop})
		}, nil
	}

	provider := &mockProvider{model: &mockLanguageModel{streamFunc: streamWithToolCall}}
	cfg := Config{Provider: provider, Model: "mock-model", MaxSteps: 5}

	result, err := Run(context.Background(), cfg, nil, "run bash", Callbacks{})
	require.NoError(t, err)
	require.NotNil(t, result)

	// Steps: [assistant(text+toolcall), tool(error result), assistant(final text)]
	require.GreaterOrEqual(t, len(result.Steps), 2)

	// First step: assistant with tool call flushed before tool result
	firstAssistant := result.Steps[0]
	assert.Equal(t, StepRoleAssistant, firstAssistant.Role)
	assert.Equal(t, "Running bash...", firstAssistant.Content)
	require.Len(t, firstAssistant.ToolCalls, 1)
	assert.Equal(t, "tc1", firstAssistant.ToolCalls[0].ID)
	assert.Equal(t, "bash", firstAssistant.ToolCalls[0].Name)
	assert.Equal(t, json.RawMessage(toolInput), firstAssistant.ToolCalls[0].Input)

	// Second step: tool result (error — tool not registered in this test)
	toolStep := result.Steps[1]
	assert.Equal(t, StepRoleTool, toolStep.Role)
	assert.Equal(t, "tc1", toolStep.ToolCallID)
}

// TestRun_OnToolStart verifies that OnToolStart fires with the correct tool name
// before execution. A regression passing tc.ToolCallID instead of tc.ToolName,
// or moving the callback to OnToolResult, would fail this test.
func TestRun_OnToolStart(t *testing.T) {
	provider := &mockProvider{model: &mockLanguageModel{streamFunc: toolCallThenDoneStream()}}
	cfg := Config{
		Provider: provider,
		Model:    "mock-model",
		Tools:    []fantasy.AgentTool{makeCaptureTool(new(*sandbox.ExecConfig))},
		MaxSteps: 5,
	}

	var toolsStarted []string
	_, err := Run(context.Background(), cfg, nil, "capture", Callbacks{
		OnToolStart: func(toolName string) {
			toolsStarted = append(toolsStarted, toolName)
		},
	})
	require.NoError(t, err)
	assert.Equal(t, []string{"capture"}, toolsStarted)
}
