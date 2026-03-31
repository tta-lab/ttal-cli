package cmd

import (
	"context"
	"fmt"
	"os"
	"runtime"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"github.com/tta-lab/logos"
	"github.com/tta-lab/ttal-cli/internal/ask"
	"github.com/tta-lab/ttal-cli/internal/config"
	"github.com/tta-lab/ttal-cli/internal/project"
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
	project    string
	repo       string
}

var subagentRunCmd = &cobra.Command{
	Use:   "run <name> <prompt>",
	Short: "Execute a subagent by name using its ttal: frontmatter config",
	Long: `Run a named subagent using model/access/system-prompt from its ttal: frontmatter.
Sandbox paths are loaded from sandbox.toml. CWD access is controlled by the agent's
ttal.access field: "rw" for read-write, "ro" for read-only.

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

// commandsForAccess returns the command docs appropriate for the given access level.
// "rw" agents get AllCommands + src edit operations; "ro" agents get AllCommands only.
func commandsForAccess(access string) []logos.CommandDoc {
	if access == "rw" {
		return ask.RWCommands()
	}
	return ask.AllCommands()
}

// buildSandboxPaths constructs AllowedPaths from sandbox.toml + CWD.
// allowWrite paths → rw, allowRead paths → ro, CWD → rw/ro per access field.
// Paths appearing in both lists are deduplicated (rw wins).
func buildSandboxPaths(sandbox *config.SandboxConfig, cwd, access string) []logos.AllowedPath {
	cwdReadOnly := access != "rw"

	// Build a deduplicated map: path → readOnly. RW wins over RO.
	seen := make(map[string]bool) // true = readOnly
	var ordered []string

	addPath := func(p string, readOnly bool) {
		if existing, ok := seen[p]; ok {
			// RW wins — upgrade ro to rw if needed.
			if existing && !readOnly {
				seen[p] = false
			}
			return
		}
		seen[p] = readOnly
		ordered = append(ordered, p)
	}

	for _, p := range sandbox.ExpandedAllowWrite() {
		addPath(p, false)
	}
	for _, p := range sandbox.ExpandedAllowRead() {
		addPath(p, true)
	}
	// CWD goes last (may upgrade an existing entry or add a new one).
	addPath(cwd, cwdReadOnly)

	paths := make([]logos.AllowedPath, 0, len(ordered))
	for _, p := range ordered {
		paths = append(paths, logos.AllowedPath{Path: p, ReadOnly: seen[p]})
	}
	return paths
}

// resolveCWD returns the working directory for subagent run based on flags.
//
//   - --project <alias> → registered project path (access from agent frontmatter)
//   - --repo <ref>      → local clone path (always ro)
//   - neither           → os.Getwd()
//
// Returns cwd, effectiveAccess, and any error.
func resolveCWD(ctx context.Context, ttalCfg *config.Config, agentAccess string) (
	cwd, effectiveAccess string, err error,
) {
	switch {
	case subagentRunFlags.project != "" && subagentRunFlags.repo != "":
		return "", "", fmt.Errorf("--project and --repo are mutually exclusive")
	case subagentRunFlags.project != "":
		p, resolveErr := project.GetProjectPath(subagentRunFlags.project)
		if resolveErr != nil {
			return "", "", fmt.Errorf("resolve project: %w", resolveErr)
		}
		return p, agentAccess, nil
	case subagentRunFlags.repo != "":
		cloneURL, localPath, resolveErr := ask.ResolveRepoRef(subagentRunFlags.repo, ttalCfg.AskReferencesPath())
		if resolveErr != nil {
			return "", "", fmt.Errorf("resolve repo: %w", resolveErr)
		}
		if ensureErr := ask.EnsureRepo(ctx, cloneURL, localPath); ensureErr != nil {
			return "", "", fmt.Errorf("ensure repo: %w", ensureErr)
		}
		return localPath, "ro", nil // repos are always read-only
	default:
		wd, wdErr := os.Getwd()
		if wdErr != nil {
			return "", "", fmt.Errorf("get working directory: %w", wdErr)
		}
		return wd, agentAccess, nil
	}
}

// claudeMDInstruction is appended to every subagent's system prompt.
// It instructs the agent to discover and read CLAUDE.md / AGENTS.md conventions.
const claudeMDInstruction = `

## Project Conventions

Before starting work, check for CLAUDE.md and AGENTS.md in the project root and subfolders. If found,
read them — they contain project conventions, architecture notes, and coding guidelines you must follow.`

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

	if agent.Frontmatter.Ttal == nil {
		return fmt.Errorf("agent %q has no ttal: block — add 'ttal: access: ro' or 'ttal: access: rw' to its frontmatter", name) //nolint:lll
	}

	access := agent.Frontmatter.Ttal.Access
	if access != "ro" && access != "rw" {
		return fmt.Errorf("agent %q has invalid access %q (want ro or rw)", name, access)
	}

	model := agent.Frontmatter.Ttal.Model
	if model == "" {
		model = ttalCfg.AskModel()
	}

	provider, modelID, err := ask.BuildProvider(model)
	if err != nil {
		return fmt.Errorf("build provider: %w", err)
	}

	cwd, effectiveAccess, err := resolveCWD(cmd.Context(), ttalCfg, access)
	if err != nil {
		return err
	}

	commands := commandsForAccess(effectiveAccess)

	promptData := logos.PromptData{
		WorkingDir: cwd,
		Platform:   runtime.GOOS,
		Date:       time.Now().Format("2006-01-02"),
		Commands:   commands,
	}
	systemPrompt, err := logos.BuildSystemPrompt(promptData)
	if err != nil {
		return fmt.Errorf("build system prompt: %w", err)
	}
	if agent.Body != "" {
		systemPrompt += "\n\n" + agent.Body
	}
	systemPrompt += claudeMDInstruction

	tc, err := ask.NewTemenosClient(context.Background())
	if err != nil {
		return err
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

	sandbox := config.LoadSandbox()
	allowedPaths := buildSandboxPaths(sandbox, cwd, effectiveAccess)

	printAgentHeader(agent.Frontmatter.Emoji, name)

	cfg := logos.Config{
		Provider:     provider,
		Model:        modelID,
		SystemPrompt: systemPrompt,
		MaxSteps:     maxSteps,
		MaxTokens:    maxTokens,
		Temenos:      tc,
		SandboxEnv:   envMap,
		AllowedPaths: allowedPaths,
	}

	result, err := logos.Run(context.Background(), cfg, nil, prompt, logos.Callbacks{
		OnDelta: renderDelta,
	})
	return flushAgentResult(result, err)
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
