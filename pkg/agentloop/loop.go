package agentloop

import (
	"context"
	"encoding/json"
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"charm.land/fantasy"
	"github.com/tta-lab/ttal-cli/pkg/agentloop/sandbox"
)

// StepRole represents the role of a message step in the agent loop.
type StepRole string

const (
	StepRoleAssistant StepRole = "assistant"
	StepRoleTool      StepRole = "tool"
)

// DefaultMaxSteps is the fallback max steps when Config.MaxSteps is 0.
const DefaultMaxSteps = 30

// DefaultMaxTokens is the fallback max output tokens when Config.MaxTokens is 0.
const DefaultMaxTokens = 16384

// Config holds everything needed to run one agent loop iteration.
type Config struct {
	Provider      fantasy.Provider
	Model         string
	SystemPrompt  string
	Tools         []fantasy.AgentTool
	MaxSteps      int      // 0 means use default (DefaultMaxSteps)
	MaxTokens     int      // 0 means use default (DefaultMaxTokens)
	SandboxEnv    []string // passed to sandbox ExecConfig
	AllowedPaths  []string // absolute dirs the read/glob/grep tools may access
	TreeThreshold int      // chars — content above this returns tree by default; 0 = use default (5000)
}

// StepMessage represents one message generated during the agent loop.
// Richer than fantasy.Message — includes tool call metadata for persistence.
type StepMessage struct {
	Role       StepRole       // StepRoleAssistant or StepRoleTool
	Content    string         // text content
	ToolCalls  []ToolCallInfo // for assistant messages with tool use
	ToolCallID string         // for tool result messages
	Timestamp  time.Time
}

// ToolCallInfo carries tool call metadata for persistence.
type ToolCallInfo struct {
	ID    string
	Name  string
	Input json.RawMessage // JSON-encoded tool input
}

// RunResult contains the agent's output after a loop completes.
type RunResult struct {
	Response string        // final text response (accumulated assistant text)
	Steps    []StepMessage // all messages generated (for persistence by caller)
	Result   *fantasy.AgentResult
}

// Callbacks holds optional streaming callbacks for the agent loop.
// All fields are nil-safe — unset callbacks are simply not called.
type Callbacks struct {
	// OnDelta is called with each text delta as Claude streams its response.
	OnDelta func(text string)
	// OnToolStart is called when the agent selects a tool, before execution begins.
	// Fires in the order: OnToolStart → tool executes → OnToolResult (internal).
	// Use this to show progress indicators (e.g. "Using flicknote…").
	OnToolStart func(toolName string)
}

// Run executes one agent loop: prompt → LLM → tool calls → response.
// Stateless — no DB, no conversation persistence. The caller handles that.
// cbs carries optional streaming callbacks; zero value disables all callbacks.
func Run(
	ctx context.Context,
	cfg Config,
	history []fantasy.Message,
	prompt string,
	cbs Callbacks,
) (*RunResult, error) {
	if cfg.Provider == nil {
		return nil, fmt.Errorf("agentloop: Config.Provider must not be nil")
	}

	// Validate and convert AllowedPaths into sandbox mounts.
	var mounts []sandbox.Mount
	for _, p := range cfg.AllowedPaths {
		if p == "" || !filepath.IsAbs(p) {
			return nil, fmt.Errorf("agentloop: AllowedPaths entry %q must be a non-empty absolute path", p)
		}
		mounts = append(mounts, sandbox.Mount{Source: p, Target: p, ReadOnly: true})
	}
	execCfg := &sandbox.ExecConfig{Env: cfg.SandboxEnv, MountDirs: mounts}
	ctx = sandbox.ContextWithExecConfig(ctx, execCfg)

	model, err := cfg.Provider.LanguageModel(ctx, cfg.Model)
	if err != nil {
		return nil, fmt.Errorf("get language model: %w", err)
	}

	maxSteps := cfg.MaxSteps
	if maxSteps <= 0 {
		maxSteps = DefaultMaxSteps
	}
	maxTokens := int64(cfg.MaxTokens)
	if maxTokens <= 0 {
		maxTokens = DefaultMaxTokens
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
			if cbs.OnDelta != nil {
				cbs.OnDelta(text)
			}
			return nil
		},

		OnToolCall: func(tc fantasy.ToolCallContent) error {
			currentToolCalls = append(currentToolCalls, ToolCallInfo{
				ID:    tc.ToolCallID,
				Name:  tc.ToolName,
				Input: json.RawMessage(tc.Input),
			})
			if cbs.OnToolStart != nil {
				cbs.OnToolStart(tc.ToolName)
			}
			return nil
		},

		OnToolResult: func(tr fantasy.ToolResultContent) error {
			// Flush current assistant text + tool calls before tool result.
			if currentText.Len() > 0 || len(currentToolCalls) > 0 {
				steps = append(steps, StepMessage{
					Role:      StepRoleAssistant,
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
				Role:       StepRoleTool,
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
			Role:      StepRoleAssistant,
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
