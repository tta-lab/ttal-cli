package agentloop

import (
	"context"
	"testing"

	"charm.land/fantasy"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
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
func (m *mockLanguageModel) StreamObject(_ context.Context, _ fantasy.ObjectCall) (fantasy.ObjectStreamResponse, error) {
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
	result, err := Run(context.Background(), cfg, nil, "say hello", func(text string) {
		deltas = append(deltas, text)
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

	result, err := Run(context.Background(), cfg, nil, "hello", nil)

	require.NoError(t, err)
	require.NotNil(t, result)
	// The final assistant text should be flushed into Steps
	require.Len(t, result.Steps, 1)
	assert.Equal(t, "assistant", result.Steps[0].Role)
	assert.Equal(t, "hello world", result.Steps[0].Content)
}

func TestRun_SandboxEnvInContext(t *testing.T) {
	// Verify that SandboxEnv is wired into context (ExecConfig check)
	// The mock model just streams text so no actual sandbox exec happens,
	// but we verify Run() completes without error when SandboxEnv is set.
	provider := &mockProvider{model: &mockLanguageModel{}}

	cfg := Config{
		Provider:   provider,
		Model:      "mock-model",
		SandboxEnv: []string{"MY_VAR=test"},
	}

	result, err := Run(context.Background(), cfg, nil, "hello", nil)
	require.NoError(t, err)
	assert.Equal(t, "hello world", result.Response)
}

func TestRun_DefaultMaxSteps(t *testing.T) {
	// When MaxSteps=0, defaults to 20 — verify no panic/error
	provider := &mockProvider{model: &mockLanguageModel{}}
	cfg := Config{Provider: provider, Model: "mock-model"}
	_, err := Run(context.Background(), cfg, nil, "hello", nil)
	require.NoError(t, err)
}
