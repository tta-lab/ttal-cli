package cmd

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"
	"github.com/tta-lab/ttal-cli/internal/ask"
	"github.com/tta-lab/ttal-cli/internal/config"
	"github.com/tta-lab/ttal-cli/internal/daemon"
	internalsync "github.com/tta-lab/ttal-cli/internal/sync"
)

var subagentCmd = &cobra.Command{
	Use:   "subagent",
	Short: "Run agent loops via daemon proxy",
}

var subagentRunFlags struct {
	maxSteps   int
	maxTokens  int
	sandboxEnv []string
	project    string
	repo       string
}

var subagentRunCmd = &cobra.Command{
	Use:   "run <name> <prompt>",
	Short: "Execute a subagent by name using its ttal: frontmatter config",
	Long: `Run a named subagent using model/access/system-prompt from its ttal: frontmatter.
Sandbox paths are loaded from sandbox.toml. CWD access is controlled by the agent's
ttal.access field: "rw" for read-write, "ro" for read-only.

The agent loop runs server-side in the daemon (where temenos is reachable),
so this command works from sandboxed worker sessions.

Examples:
  ttal subagent run pr-code-reviewer "Review the current diff"
  ttal subagent run coder "implement the foo function in bar.go"
  ttal subagent run coder "implement X" --project ttal-cli
  ttal subagent run pr-code-reviewer "Review PR" --repo woodpecker-ci/woodpecker`,
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

func runSubagentByName(cmd *cobra.Command, args []string) error {
	name := args[0]
	prompt := args[1]

	ttalCfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}

	maxSteps, maxTokens := resolveLimits(cmd, ttalCfg, subagentRunFlags.maxSteps, subagentRunFlags.maxTokens)

	// Convert --env KEY=VALUE flags to map.
	envMap := make(map[string]string, len(subagentRunFlags.sandboxEnv))
	for _, e := range subagentRunFlags.sandboxEnv {
		k, v, ok := strings.Cut(e, "=")
		if !ok {
			return fmt.Errorf("invalid --env value %q: expected KEY=VALUE\n\n  Example: ttal subagent run myagent --env FOO=bar --env BAZ=qux", e) //nolint:lll
		}
		envMap[k] = v
	}

	// Resolve working dir client-side (default = os.Getwd).
	workingDir := ""
	if subagentRunFlags.project == "" && subagentRunFlags.repo == "" {
		wd, wdErr := os.Getwd()
		if wdErr != nil {
			return fmt.Errorf("get working directory: %w", wdErr)
		}
		workingDir = wd
	}

	// Look up the agent locally for the header display (emoji + name).
	// This is a cheap local FS scan — no temenos needed.
	if agent, findErr := ask.FindAgent(name, ttalCfg.Sync.SubagentsPaths); findErr == nil {
		printAgentHeader(agent.Frontmatter.Emoji, name)
	}

	req := ask.SubagentRequest{
		Name:       name,
		Prompt:     prompt,
		Project:    subagentRunFlags.project,
		Repo:       subagentRunFlags.repo,
		MaxSteps:   maxSteps,
		MaxTokens:  maxTokens,
		SandboxEnv: envMap,
		WorkingDir: workingDir,
	}

	var agentErr string

	eventHandler, sp := buildAskEventCallbacks(false)
	defer sp.Stop()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	err = daemon.RunSubagent(ctx, req, func(event ask.Event) {
		eventHandler(event)
		if event.Type == ask.EventError {
			agentErr = event.Message
		}
	})
	if err != nil {
		return err // transport/daemon error
	}

	if agentErr != "" {
		return fmt.Errorf("agent: %s", agentErr)
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

	// Compute column widths.
	nameW, accessW, modelW := len("NAME"), len("ACCESS"), len("MODEL")
	for _, a := range agents {
		if a.Frontmatter.Ttal == nil {
			continue
		}
		if n := len(a.Frontmatter.Name); n > nameW {
			nameW = n
		}
		if n := len(a.Frontmatter.Ttal.Access); n > accessW {
			accessW = n
		}
		m := a.Frontmatter.Ttal.Model
		if m == "" {
			m = "(default)"
		}
		if n := len(m); n > modelW {
			modelW = n
		}
	}

	// Header.
	fmt.Printf("%-*s  %-*s  %-*s  %s\n", nameW, "NAME", accessW, "ACCESS", modelW, "MODEL", "DESCRIPTION")
	for _, a := range agents {
		if a.Frontmatter.Ttal == nil {
			continue
		}
		m := a.Frontmatter.Ttal.Model
		if m == "" {
			m = "(default)"
		}
		desc := firstLine(a.Frontmatter.Description)
		fmt.Printf("%-*s  %-*s  %-*s  %s\n", nameW, a.Frontmatter.Name, accessW, a.Frontmatter.Ttal.Access, modelW, m, desc)
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
	subagentRunCmd.Flags().StringVar(&subagentRunFlags.project, "project", "", "Run against a registered ttal project")         //nolint:lll
	subagentRunCmd.Flags().StringVar(&subagentRunFlags.repo, "repo", "", "Run against a GitHub/Forgejo repo (auto-clone/pull)") //nolint:lll

	subagentCmd.AddCommand(subagentRunCmd)
	subagentCmd.AddCommand(subagentListCmd)
	rootCmd.AddCommand(subagentCmd)
}
