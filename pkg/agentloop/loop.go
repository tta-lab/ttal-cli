package agentloop

import (
	"context"
	"fmt"
	"strings"
	"time"

	"charm.land/fantasy"
	"github.com/tta-lab/ttal-cli/pkg/agentloop/sandbox"
)

// Config holds everything needed to run one agent loop iteration.
type Config struct {
	Provider     fantasy.Provider
	Model        string
	SystemPrompt string
	Tools        []fantasy.AgentTool
	MaxSteps     int
	MaxTokens    int
	SandboxEnv   []string // passed to sandbox ExecConfig
}

// StepMessage represents one message generated during the agent loop.
// Richer than fantasy.Message — includes tool call metadata for persistence.
type StepMessage struct {
	Role       string         // "assistant" or "tool"
	Content    string         // text content
	ToolCalls  []ToolCallInfo // for assistant messages with tool use
	ToolCallID string         // for tool result messages
	Timestamp  time.Time
}

// ToolCallInfo carries tool call metadata for persistence.
type ToolCallInfo struct {
	ID    string
	Name  string
	Input string // JSON string as returned by fantasy
}

// RunResult contains the agent's output after a loop completes.
type RunResult struct {
	Response string        // final text response (accumulated assistant text)
	Steps    []StepMessage // all messages generated (for persistence by caller)
	Result   *fantasy.AgentResult
}

// Run executes one agent loop: prompt → LLM → tool calls → response.
// Stateless — no DB, no conversation persistence. The caller handles that.
// onDelta is called with each text delta as it streams; pass nil to disable streaming.
func Run(ctx context.Context, cfg Config, history []fantasy.Message, prompt string, onDelta func(text string)) (*RunResult, error) {
	// Wire sandbox env into context so tools can access it.
	execCfg := &sandbox.ExecConfig{Env: cfg.SandboxEnv}
	ctx = sandbox.ContextWithExecConfig(ctx, execCfg)

	model, err := cfg.Provider.LanguageModel(ctx, cfg.Model)
	if err != nil {
		return nil, fmt.Errorf("get language model: %w", err)
	}

	maxSteps := cfg.MaxSteps
	if maxSteps <= 0 {
		maxSteps = 20
	}
	maxTokens := int64(cfg.MaxTokens)
	if maxTokens <= 0 {
		maxTokens = 4096
	}

	agnt := fantasy.NewAgent(model,
		fantasy.WithSystemPrompt(cfg.SystemPrompt),
		fantasy.WithTools(cfg.Tools...),
		fantasy.WithMaxOutputTokens(maxTokens),
		fantasy.WithStopConditions(fantasy.StepCountIs(maxSteps)),
	)

	var (
		steps            []StepMessage
		responseText     strings.Builder
		currentText      strings.Builder
		currentToolCalls []ToolCallInfo
	)

	result, streamErr := agnt.Stream(ctx, fantasy.AgentStreamCall{
		Messages: history,
		Prompt:   prompt,

		OnTextDelta: func(id, text string) error {
			currentText.WriteString(text)
			responseText.WriteString(text)
			if onDelta != nil {
				onDelta(text)
			}
			return nil
		},

		OnToolCall: func(tc fantasy.ToolCallContent) error {
			currentToolCalls = append(currentToolCalls, ToolCallInfo{
				ID:    tc.ToolCallID,
				Name:  tc.ToolName,
				Input: tc.Input,
			})
			return nil
		},

		OnToolResult: func(tr fantasy.ToolResultContent) error {
			// Flush current assistant text + tool calls before tool result.
			if currentText.Len() > 0 || len(currentToolCalls) > 0 {
				steps = append(steps, StepMessage{
					Role:      "assistant",
					Content:   currentText.String(),
					ToolCalls: currentToolCalls,
					Timestamp: time.Now().UTC(),
				})
				currentText.Reset()
				currentToolCalls = nil
			}

			resultText := ""
			if text, ok := fantasy.AsToolResultOutputType[fantasy.ToolResultOutputContentText](tr.Result); ok {
				resultText = text.Text
			} else {
				resultText = fmt.Sprintf("[non-text result: %s]", tr.Result.GetType())
			}

			steps = append(steps, StepMessage{
				Role:       "tool",
				ToolCallID: tr.ToolCallID,
				Content:    resultText,
				Timestamp:  time.Now().UTC(),
			})
			return nil
		},
	})

	// Flush any remaining assistant text after the loop.
	if currentText.Len() > 0 {
		steps = append(steps, StepMessage{
			Role:      "assistant",
			Content:   currentText.String(),
			ToolCalls: currentToolCalls,
			Timestamp: time.Now().UTC(),
		})
	}

	return &RunResult{
		Response: responseText.String(),
		Steps:    steps,
		Result:   result,
	}, streamErr
}
