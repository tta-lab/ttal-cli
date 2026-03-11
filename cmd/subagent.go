package cmd

import (
	"context"
	"fmt"
	"os"
	"strings"

	"charm.land/fantasy"
	"charm.land/fantasy/providers/anthropic"
	"github.com/spf13/cobra"
	"github.com/tta-lab/ttal-cli/pkg/agentloop"
	"github.com/tta-lab/ttal-cli/pkg/agentloop/sandbox"
	"github.com/tta-lab/ttal-cli/pkg/agentloop/tools"
)

var subagentCmd = &cobra.Command{
	Use:   "subagent",
	Short: "Run agent loops locally",
}

var subagentRunFlags struct {
	provider     string
	model        string
	systemPrompt string
	toolNames    []string
	maxSteps     int
	maxTokens    int
	sandboxEnv   []string
}

var subagentRunCmd = &cobra.Command{
	Use:   "run <prompt>",
	Short: "Execute a one-shot agent loop",
	Long: `Run a stateless agent loop with the given prompt. Streams output to stdout.

Examples:
  ttal subagent run "Fetch https://example.com and summarize it"
  ttal subagent run --tool bash --tool web_search "Search for Go generics tutorials"
  ttal subagent run --system "You are a code reviewer" "Review the diff at /tmp/diff.txt"`,
	Args: cobra.ExactArgs(1),
	RunE: runSubagent,
}

func runSubagent(cmd *cobra.Command, args []string) error {
	prompt := args[0]

	provider, err := buildProvider(subagentRunFlags.provider)
	if err != nil {
		return fmt.Errorf("build provider: %w", err)
	}

	sbx := &sandbox.Sandbox{
		BwrapPath:        "bwrap",
		AllowUnsandboxed: true, // local dev — bwrap may not be available
	}

	fetchBackend := tools.NewDefuddleCLIBackend()
	allTools := tools.NewDefaultToolSet(sbx, fetchBackend)
	selectedTools := filterTools(allTools, subagentRunFlags.toolNames)

	cfg := agentloop.Config{
		Provider:     provider,
		Model:        subagentRunFlags.model,
		SystemPrompt: subagentRunFlags.systemPrompt,
		Tools:        selectedTools,
		MaxSteps:     subagentRunFlags.maxSteps,
		MaxTokens:    subagentRunFlags.maxTokens,
		SandboxEnv:   subagentRunFlags.sandboxEnv,
	}

	result, err := agentloop.Run(context.Background(), cfg, nil, prompt, func(text string) {
		fmt.Print(text)
	})
	if err != nil {
		return fmt.Errorf("agent loop: %w", err)
	}

	// Ensure trailing newline if the model didn't emit one.
	if result.Response != "" && !strings.HasSuffix(result.Response, "\n") {
		fmt.Println()
	}

	return nil
}

// buildProvider creates a fantasy.Provider from the provider flag.
// Currently supports "anthropic" using ANTHROPIC_API_KEY.
func buildProvider(providerName string) (fantasy.Provider, error) {
	switch providerName {
	case "anthropic":
		apiKey := os.Getenv("ANTHROPIC_API_KEY")
		if apiKey == "" {
			return nil, fmt.Errorf("ANTHROPIC_API_KEY is not set")
		}
		return anthropic.New(anthropic.WithAPIKey(apiKey))
	default:
		return nil, fmt.Errorf("unsupported provider %q — currently only \"anthropic\" is supported", providerName)
	}
}

// filterTools returns all tools if names is empty, otherwise only the named tools.
func filterTools(allTools []fantasy.AgentTool, names []string) []fantasy.AgentTool {
	if len(names) == 0 {
		return allTools
	}
	selected := make([]fantasy.AgentTool, 0, len(names))
	for _, tool := range allTools {
		for _, name := range names {
			if tool.Info().Name == name {
				selected = append(selected, tool)
				break
			}
		}
	}
	return selected
}

func init() {
	subagentRunCmd.Flags().StringVar(&subagentRunFlags.provider, "provider", "anthropic", "LLM provider (anthropic)")
	subagentRunCmd.Flags().StringVar(&subagentRunFlags.model, "model", "claude-sonnet-4-6", "Model ID")
	subagentRunCmd.Flags().StringVar(
		&subagentRunFlags.systemPrompt, "system", "You are a helpful assistant.", "System prompt",
	)
	subagentRunCmd.Flags().StringArrayVar(
		&subagentRunFlags.toolNames, "tool", nil, "Tools to enable (bash, web_fetch, web_search); default: all",
	)
	subagentRunCmd.Flags().IntVar(&subagentRunFlags.maxSteps, "max-steps", 20, "Maximum agent steps")
	subagentRunCmd.Flags().IntVar(&subagentRunFlags.maxTokens, "max-tokens", 4096, "Maximum output tokens per step")
	subagentRunCmd.Flags().StringArrayVar(
		&subagentRunFlags.sandboxEnv, "env", nil, "Extra env vars for sandbox (KEY=VALUE)",
	)

	subagentCmd.AddCommand(subagentRunCmd)
	rootCmd.AddCommand(subagentCmd)
}
