package cmd

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"time"

	"charm.land/fantasy"
	"charm.land/fantasy/providers/anthropic"
	"github.com/spf13/cobra"
	"github.com/tta-lab/ttal-cli/internal/config"
	internalsync "github.com/tta-lab/ttal-cli/internal/sync"
	"github.com/tta-lab/ttal-cli/pkg/agentloop"
	"github.com/tta-lab/ttal-cli/pkg/agentloop/sandbox"
	"github.com/tta-lab/ttal-cli/pkg/agentloop/tools"
)

var subagentCmd = &cobra.Command{
	Use:   "subagent",
	Short: "Run agent loops locally",
}

var subagentRunFlags struct {
	maxSteps      int
	maxTokens     int
	sandboxEnv    []string
	treeThreshold int
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
func findTtalAgent(name string) (*internalsync.ParsedAgent, error) {
	paths, err := loadSubagentsPaths()
	if err != nil {
		return nil, err
	}
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

const scrapesCacheDir = "~/.ttal/scrapes"

// resolveFetchBackendFor returns a fetch backend only if the tool list includes URL tools
// (read_url or search_web). Otherwise returns a lightweight placeholder that satisfies the
// interface but is never invoked (since those tools are filtered out).
func resolveFetchBackendFor(toolNames []string) (tools.ReadURLBackend, error) {
	for _, name := range toolNames {
		if name == "read_url" || name == "search_web" {
			return resolveFetchBackend()
		}
	}
	return tools.NewDefuddleCLIBackend(), nil // placeholder: never called when URL tools absent
}

// resolveFetchBackend returns the best available URL fetch backend:
//  1. defuddle installed → CachedFetchBackend wrapping DefuddleCLIBackend
//  2. BROWSER_GATEWAY_URL set → CachedFetchBackend wrapping BrowserGatewayBackend
//  3. Neither → error (fail fast)
func resolveFetchBackend() (tools.ReadURLBackend, error) {
	cacheDir := config.ExpandHome(scrapesCacheDir)

	if _, err := exec.LookPath("defuddle"); err == nil {
		return tools.NewCachedFetchBackend(cacheDir, tools.NewDefuddleCLIBackend()), nil
	}
	if gwURL := os.Getenv("BROWSER_GATEWAY_URL"); gwURL != "" {
		return tools.NewCachedFetchBackend(cacheDir, tools.NewBrowserGatewayBackend(gwURL, nil)), nil
	}
	return nil, fmt.Errorf("no fetch backend available: install defuddle or set BROWSER_GATEWAY_URL")
}

// buildToolSet creates and filters the tool set for a subagent.
func buildToolSet(toolNames, allowedPaths []string, fetchBackend tools.ReadURLBackend) ([]fantasy.AgentTool, error) {
	sbx := sandbox.New(sandbox.Options{AllowUnsandboxed: true})
	allTools := tools.NewDefaultToolSet(sbx, fetchBackend, allowedPaths, subagentRunFlags.treeThreshold)
	return filterTools(allTools, toolNames)
}

// buildAgentSystemPrompt constructs the system prompt for a subagent, appending the agent body.
func buildAgentSystemPrompt(
	cwd string, allowedPaths []string, selectedTools []fantasy.AgentTool, agentBody string,
) (string, error) {
	richDescs := tools.RichToolDescriptions(selectedTools)
	toolInfos := make([]agentloop.ToolInfo, len(richDescs))
	for i, d := range richDescs {
		toolInfos[i] = agentloop.ToolInfo{Name: d.Name, Description: d.Description}
	}
	base, err := agentloop.BuildSystemPrompt(agentloop.PromptData{
		WorkingDir:   cwd,
		Platform:     runtime.GOOS,
		Date:         time.Now().Format("2006-01-02"),
		AllowedPaths: allowedPaths,
		Tools:        toolInfos,
	})
	if err != nil {
		return "", fmt.Errorf("build system prompt: %w", err)
	}
	if agentBody == "" {
		return base, nil
	}
	return base + "\n\n" + agentBody, nil
}

func runSubagentByName(cmd *cobra.Command, args []string) error {
	name := args[0]
	prompt := args[1]

	agent, err := findTtalAgent(name)
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
	allowedPaths := []string{cwd}

	fetchBackend, err := resolveFetchBackendFor(agent.Frontmatter.Ttal.Tools)
	if err != nil {
		return fmt.Errorf("resolve fetch backend: %w", err)
	}

	selectedTools, err := buildToolSet(agent.Frontmatter.Ttal.Tools, allowedPaths, fetchBackend)
	if err != nil {
		return err
	}

	systemPrompt, err := buildAgentSystemPrompt(cwd, allowedPaths, selectedTools, agent.Body)
	if err != nil {
		return err
	}

	cfg := agentloop.Config{
		Provider:      provider,
		Model:         modelID,
		SystemPrompt:  systemPrompt,
		Tools:         selectedTools,
		MaxSteps:      subagentRunFlags.maxSteps,
		MaxTokens:     subagentRunFlags.maxTokens,
		SandboxEnv:    subagentRunFlags.sandboxEnv,
		AllowedPaths:  allowedPaths,
		TreeThreshold: subagentRunFlags.treeThreshold,
	}

	result, err := agentloop.Run(context.Background(), cfg, nil, prompt, func(text string) {
		fmt.Print(text)
	})
	if err != nil {
		return fmt.Errorf("agent loop: %w", err)
	}

	if result.Response != "" && !strings.HasSuffix(result.Response, "\n") {
		fmt.Println()
	}

	return nil
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
	subagentRunCmd.Flags().IntVar(&subagentRunFlags.maxSteps, "max-steps", 20, "Maximum agent steps")
	subagentRunCmd.Flags().IntVar(&subagentRunFlags.maxTokens, "max-tokens", 4096, "Maximum output tokens per step")
	subagentRunCmd.Flags().StringArrayVar(
		&subagentRunFlags.sandboxEnv, "env", nil, "Extra env vars for sandbox (KEY=VALUE)",
	)
	subagentRunCmd.Flags().IntVar(
		&subagentRunFlags.treeThreshold, "tree-threshold", 5000,
		"Char count above which read_md and read_url default to tree view",
	)

	subagentCmd.AddCommand(subagentRunCmd)
	subagentCmd.AddCommand(subagentListCmd)
	rootCmd.AddCommand(subagentCmd)
}
