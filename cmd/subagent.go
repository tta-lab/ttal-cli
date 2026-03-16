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
	"github.com/tta-lab/logos"
	"github.com/tta-lab/ttal-cli/internal/config"
	internalsync "github.com/tta-lab/ttal-cli/internal/sync"
)

var subagentCmd = &cobra.Command{
	Use:   "subagent",
	Short: "Run agent loops locally",
}

var subagentRunFlags struct {
	maxSteps   int
	maxTokens  int
	sandboxEnv []string
}

var subagentRunCmd = &cobra.Command{
	Use:   "run <name> <prompt>",
	Short: "Execute a subagent by name using its ttal: frontmatter config",
	Long: `Run a named subagent using model/tools/system-prompt from its ttal: frontmatter.
CWD is automatically added to allowed paths.

Examples:
  ttal subagent run pr-code-reviewer "Review the current diff"
  ttal subagent run web-fetcher "Fetch https://example.com and summarize"`,
	Args: cobra.ExactArgs(2),
	RunE: runSubagentByName,
}

var subagentListCmd = &cobra.Command{
	Use:   "list",
	Short: "List subagents that have ttal: frontmatter config",
	RunE:  listSubagents,
}

func loadSubagentsPaths() ([]string, error) {
	cfg, err := config.Load()
	if err != nil {
		return nil, fmt.Errorf("load config: %w", err)
	}
	return cfg.Sync.SubagentsPaths, nil
}

// findTtalAgent discovers ttal-configured agents and returns the one matching name.
func findTtalAgent(name string, paths []string) (*internalsync.ParsedAgent, error) {
	agents, err := internalsync.DiscoverTtalAgents(paths)
	if err != nil {
		return nil, fmt.Errorf("discover agents: %w", err)
	}
	for _, a := range agents {
		if a.Frontmatter.Name == name {
			return a, nil
		}
	}
	available := make([]string, len(agents))
	for i, a := range agents {
		available[i] = a.Frontmatter.Name
	}
	if len(available) == 0 {
		return nil, fmt.Errorf("agent %q not found (no agents with ttal: frontmatter discovered)", name)
	}
	return nil, fmt.Errorf("agent %q not found — available: %s", name, strings.Join(available, ", "))
}

func runSubagentByName(cmd *cobra.Command, args []string) error {
	name := args[0]
	prompt := args[1]

	ttalCfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}

	agent, err := findTtalAgent(name, ttalCfg.Sync.SubagentsPaths)
	if err != nil {
		return err
	}

	model := agent.Frontmatter.Ttal.Model
	if model == "" {
		return fmt.Errorf("agent %q has no model in ttal: frontmatter", name)
	}

	provider, modelID, err := buildSubagentProvider(model)
	if err != nil {
		return fmt.Errorf("build provider: %w", err)
	}

	cwd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("get working directory: %w", err)
	}

	// Derive capability switches from frontmatter tools list.
	network, readFS := deriveCapabilities(agent.Frontmatter.Ttal.Tools)

	promptData := logos.PromptData{
		WorkingDir: cwd,
		Platform:   runtime.GOOS,
		Date:       time.Now().Format("2006-01-02"),
		Network:    network,
		ReadFS:     readFS,
	}
	systemPrompt, err := logos.BuildSystemPrompt(promptData)
	if err != nil {
		return fmt.Errorf("build system prompt: %w", err)
	}
	if agent.Body != "" {
		systemPrompt += "\n\n" + agent.Body
	}

	tc, err := logos.NewClient("")
	if err != nil {
		return fmt.Errorf("connect to temenos daemon: %w\n\n"+
			"Is the daemon running? Try: temenos daemon install && temenos daemon start", err)
	}

	maxSteps, maxTokens := resolveLimits(cmd, ttalCfg, subagentRunFlags.maxSteps, subagentRunFlags.maxTokens)

	// Convert --env KEY=VALUE flags to map.
	envMap := make(map[string]string, len(subagentRunFlags.sandboxEnv))
	for _, e := range subagentRunFlags.sandboxEnv {
		if k, v, ok := strings.Cut(e, "="); ok {
			envMap[k] = v
		}
	}

	printAgentHeader(agent.Frontmatter.Emoji, name)

	cfg := logos.Config{
		Provider:     provider,
		Model:        modelID,
		SystemPrompt: systemPrompt,
		MaxSteps:     maxSteps,
		MaxTokens:    maxTokens,
		Temenos:      tc,
		SandboxEnv:   envMap,
		AllowedPaths: []logos.AllowedPath{{Path: cwd, ReadOnly: true}},
	}

	result, err := logos.Run(context.Background(), cfg, nil, prompt, logos.Callbacks{
		OnDelta: func(text string) { fmt.Print(text) },
	})
	if result != nil && result.Response != "" && !strings.HasSuffix(result.Response, "\n") {
		fmt.Println()
	}
	if err != nil {
		return fmt.Errorf("agent loop: %w", err)
	}
	return nil
}

// deriveCapabilities maps frontmatter tool names to logos capability switches.
// If tools is empty, defaults to both capabilities enabled (full access).
func deriveCapabilities(toolNames []string) (network, readFS bool) {
	if len(toolNames) == 0 {
		return true, true // default: full access
	}
	for _, t := range toolNames {
		switch t {
		case "read_url", "search_web":
			network = true
		case "bash", "read", "read_md", "glob", "grep":
			readFS = true
		}
	}
	return network, readFS
}

func listSubagents(_ *cobra.Command, _ []string) error {
	paths, err := loadSubagentsPaths()
	if err != nil {
		return err
	}

	agents, err := internalsync.DiscoverTtalAgents(paths)
	if err != nil {
		return fmt.Errorf("discover agents: %w", err)
	}

	if len(agents) == 0 {
		fmt.Println("No subagents with ttal: frontmatter found.")
		return nil
	}

	// Compute column widths
	nameW, modelW, toolsW := len("NAME"), len("MODEL"), len("TOOLS")
	for _, a := range agents {
		if a.Frontmatter.Ttal == nil {
			continue
		}
		if n := len(a.Frontmatter.Name); n > nameW {
			nameW = n
		}
		if n := len(a.Frontmatter.Ttal.Model); n > modelW {
			modelW = n
		}
		ts := strings.Join(a.Frontmatter.Ttal.Tools, ", ")
		if n := len(ts); n > toolsW {
			toolsW = n
		}
	}

	// Header
	fmt.Printf("%-*s  %-*s  %-*s  %s\n", nameW, "NAME", modelW, "MODEL", toolsW, "TOOLS", "DESCRIPTION")
	for _, a := range agents {
		if a.Frontmatter.Ttal == nil {
			continue
		}
		ts := strings.Join(a.Frontmatter.Ttal.Tools, ", ")
		desc := firstLine(a.Frontmatter.Description)
		fmt.Printf("%-*s  %-*s  %-*s  %s\n", nameW, a.Frontmatter.Name, modelW, a.Frontmatter.Ttal.Model, toolsW, ts, desc)
	}
	return nil
}

// printAgentHeader prints a visual header with the agent emoji and name before output begins.
// If emoji is empty, the name is printed without one.
func printAgentHeader(emoji, name string) {
	if emoji != "" {
		fmt.Fprintf(os.Stderr, "\n%s %s\n\n", emoji, name)
	} else {
		fmt.Fprintf(os.Stderr, "\n%s\n\n", name)
	}
}

// firstLine returns the first non-empty line of s.
func firstLine(s string) string {
	for _, line := range strings.Split(s, "\n") {
		line = strings.TrimSpace(line)
		if line != "" {
			return line
		}
	}
	return ""
}

// buildSubagentProvider creates a fantasy.Provider and resolved model ID from a model string.
// Model format: "provider/model-id" or bare model ID (defaults to anthropic).
// Currently supports: "minimax/" prefix (→ anthropic-compat via MINIMAX_API_URL/MINIMAX_API_KEY)
// and bare model IDs (→ anthropic via ANTHROPIC_API_KEY).
func buildSubagentProvider(model string) (fantasy.Provider, string, error) {
	switch {
	case strings.HasPrefix(model, "minimax/"):
		baseURL := os.Getenv("MINIMAX_API_URL")
		apiKey := os.Getenv("MINIMAX_API_KEY")
		if baseURL == "" || apiKey == "" {
			return nil, "", fmt.Errorf("minimax/ model requires MINIMAX_API_URL and MINIMAX_API_KEY env vars")
		}
		modelID := strings.TrimPrefix(model, "minimax/")
		p, err := anthropic.New(anthropic.WithBaseURL(baseURL), anthropic.WithAPIKey(apiKey))
		if err != nil {
			return nil, "", fmt.Errorf("minimax provider (%s): %w", modelID, err)
		}
		return p, modelID, nil
	default:
		apiKey := os.Getenv("ANTHROPIC_API_KEY")
		if apiKey == "" {
			return nil, "", fmt.Errorf("ANTHROPIC_API_KEY is not set")
		}
		p, err := anthropic.New(anthropic.WithAPIKey(apiKey))
		if err != nil {
			return nil, "", fmt.Errorf("anthropic provider (%s): %w", model, err)
		}
		return p, model, nil
	}
}

// resolveLimits returns effective max steps and max tokens.
// Priority: explicit flag > config > built-in default (already baked into flagSteps/flagTokens).
func resolveLimits(cmd *cobra.Command, cfg *config.Config, flagSteps, flagTokens int) (maxSteps, maxTokens int) {
	maxSteps = flagSteps
	maxTokens = flagTokens
	if !cmd.Flags().Changed("max-steps") {
		maxSteps = cfg.AskMaxSteps()
	}
	if !cmd.Flags().Changed("max-tokens") {
		maxTokens = cfg.AskMaxTokens()
	}
	return
}

func init() {
	subagentRunCmd.Flags().IntVar(&subagentRunFlags.maxSteps, "max-steps", config.AskDefaultMaxSteps, "Maximum agent steps")               //nolint:lll
	subagentRunCmd.Flags().IntVar(&subagentRunFlags.maxTokens, "max-tokens", config.AskDefaultMaxTokens, "Maximum output tokens per step") //nolint:lll
	subagentRunCmd.Flags().StringArrayVar(
		&subagentRunFlags.sandboxEnv, "env", nil, "Extra env vars for sandbox (KEY=VALUE)",
	)

	subagentCmd.AddCommand(subagentRunCmd)
	subagentCmd.AddCommand(subagentListCmd)
	rootCmd.AddCommand(subagentCmd)
}
