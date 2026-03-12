package cmd

import (
	"context"
	"fmt"
	"os"
	"runtime"
	"strings"
	"time"

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
	provider      string
	model         string
	systemPrompt  string
	toolNames     []string
	maxSteps      int
	maxTokens     int
	sandboxEnv    []string
	allowedPaths  []string
	treeThreshold int
}

var subagentRunCmd = &cobra.Command{
	Use:   "run <prompt>",
	Short: "Execute a one-shot agent loop",
	Long: `Run a stateless agent loop with the given prompt. Streams output to stdout.

Examples:
  ttal subagent run "Fetch https://example.com and summarize it"
  ttal subagent run --tool bash --tool search_web "Search for Go generics tutorials"
  ttal subagent run --system "You are a code reviewer" "Review the diff at /tmp/diff.txt"
  ttal subagent run --allowed-path /path/to/repo "Review the code in main.go"`,
	Args: cobra.ExactArgs(1),
	RunE: runSubagent,
}

func runSubagent(cmd *cobra.Command, args []string) error {
	prompt := args[0]

	provider, err := buildProvider(subagentRunFlags.provider)
	if err != nil {
		return fmt.Errorf("build provider: %w", err)
	}

	sbx := sandbox.New(sandbox.Options{
		AllowUnsandboxed: true, // local dev — platform sandbox may not be available
	})

	fetchBackend := tools.NewDefuddleCLIBackend()
	allTools := tools.NewDefaultToolSet(sbx, fetchBackend, subagentRunFlags.allowedPaths, subagentRunFlags.treeThreshold)
	selectedTools, err := filterTools(allTools, subagentRunFlags.toolNames)
	if err != nil {
		return err
	}

	// Build dynamic system prompt from runtime context.
	richDescs := tools.RichToolDescriptions(selectedTools)
	toolInfos := make([]agentloop.ToolInfo, len(richDescs))
	for i, d := range richDescs {
		toolInfos[i] = agentloop.ToolInfo{Name: d.Name, Description: d.Description}
	}

	cwd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("get working directory: %w", err)
	}
	basePrompt, err := agentloop.BuildSystemPrompt(agentloop.PromptData{
		WorkingDir:   cwd,
		Platform:     runtime.GOOS,
		Date:         time.Now().Format("2006-01-02"),
		AllowedPaths: subagentRunFlags.allowedPaths,
		Tools:        toolInfos,
	})
	if err != nil {
		return fmt.Errorf("build system prompt: %w", err)
	}

	systemPrompt := basePrompt
	if subagentRunFlags.systemPrompt != "" {
		systemPrompt = basePrompt + "\n\n" + subagentRunFlags.systemPrompt
	}

	cfg := agentloop.Config{
		Provider:      provider,
		Model:         subagentRunFlags.model,
		SystemPrompt:  systemPrompt,
		Tools:         selectedTools,
		MaxSteps:      subagentRunFlags.maxSteps,
		MaxTokens:     subagentRunFlags.maxTokens,
		SandboxEnv:    subagentRunFlags.sandboxEnv,
		AllowedPaths:  subagentRunFlags.allowedPaths,
		TreeThreshold: subagentRunFlags.treeThreshold,
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
// Returns an error if any requested name does not match a known tool.
func filterTools(allTools []fantasy.AgentTool, names []string) ([]fantasy.AgentTool, error) {
	if len(names) == 0 {
		return allTools, nil
	}
	byName := make(map[string]fantasy.AgentTool, len(allTools))
	availableNames := make([]string, 0, len(allTools))
	for _, tool := range allTools {
		byName[tool.Info().Name] = tool
		availableNames = append(availableNames, tool.Info().Name)
	}
	selected := make([]fantasy.AgentTool, 0, len(names))
	for _, name := range names {
		tool, ok := byName[name]
		if !ok {
			return nil, fmt.Errorf("unknown tool %q — available: %s", name, strings.Join(availableNames, ", "))
		}
		selected = append(selected, tool)
	}
	return selected, nil
}

func init() {
	subagentRunCmd.Flags().StringVar(&subagentRunFlags.provider, "provider", "anthropic", "LLM provider (anthropic)")
	subagentRunCmd.Flags().StringVar(&subagentRunFlags.model, "model", "claude-sonnet-4-6", "Model ID")
	subagentRunCmd.Flags().StringVar(
		&subagentRunFlags.systemPrompt, "system", "", "Additional instructions appended to the default system prompt",
	)
	subagentRunCmd.Flags().StringArrayVar(
		&subagentRunFlags.toolNames, "tool", nil, //nolint:lll
		"Tools to enable (bash, read_url, search_web, read, read_md, glob, grep); default: all",
	)
	subagentRunCmd.Flags().IntVar(&subagentRunFlags.maxSteps, "max-steps", 20, "Maximum agent steps")
	subagentRunCmd.Flags().IntVar(&subagentRunFlags.maxTokens, "max-tokens", 4096, "Maximum output tokens per step")
	subagentRunCmd.Flags().StringArrayVar(
		&subagentRunFlags.sandboxEnv, "env", nil, "Extra env vars for sandbox (KEY=VALUE)",
	)
	subagentRunCmd.Flags().StringArrayVar(
		&subagentRunFlags.allowedPaths, "allowed-path", nil,
		"Directories the read/glob/grep tools can access (repeatable)",
	)
	subagentRunCmd.Flags().IntVar(
		&subagentRunFlags.treeThreshold, "tree-threshold", 5000,
		"Char count above which read_md and read_url default to tree view",
	)

	subagentCmd.AddCommand(subagentRunCmd)
	rootCmd.AddCommand(subagentCmd)
}
