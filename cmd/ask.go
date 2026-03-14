package cmd

import (
	"context"
	_ "embed"
	"fmt"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"github.com/tta-lab/ttal-cli/internal/config"
	"github.com/tta-lab/ttal-cli/internal/project"
	"github.com/tta-lab/ttal-cli/internal/usage"
	"github.com/tta-lab/ttal-cli/pkg/agentloop"
	"github.com/tta-lab/ttal-cli/pkg/agentloop/tools"
)

//go:embed ask_prompts/project.md
var askProjectPrompt string

//go:embed ask_prompts/repo.md
var askRepoPrompt string

//go:embed ask_prompts/url.md
var askURLPrompt string

//go:embed ask_prompts/web.md
var askWebPrompt string

//go:embed ask_prompts/general.md
var askGeneralPrompt string

// askCodespaceTools is the tool set for modes that explore a local codebase
// and may also need to fetch external docs or search the web.
var askCodespaceTools = []string{"bash", "read", "read_md", "glob", "grep", "search_web", "read_url"}

var askFlags struct {
	project   string
	repo      string
	url       string
	web       bool
	maxSteps  int
	maxTokens int
}

var askCmd = &cobra.Command{
	Use:   "ask <question>",
	Short: "Ask about code, repos, web pages, or the web using an AI agent",
	Long: `Ask a natural language question about a codebase, open-source repository, or web page.

With no flags, explores the current directory with both filesystem and web access.
Use a flag to narrow the scope to a specific source:

  --project <alias>      Ask about a registered ttal project
  --repo <url|org/repo>  Ask about a GitHub repo (auto-clone/pull)
  --url <url>            Ask about a web page (pre-fetched with defuddle)
  --web                  Search the web to answer the question

Examples:
  ttal ask "how does the auth middleware work?"                               # general (CWD + web)
  ttal ask "how does routing work?" --project ttal-cli                        # registered project
  ttal ask "how does pipeline syntax work?" --repo woodpecker-ci/woodpecker   # OSS repo
  ttal ask "what API endpoints are available?" --url https://docs.example.com # specific URL
  ttal ask "what is the latest Go generics syntax?" --web                     # web search only`,
	Args: cobra.ExactArgs(1),
	RunE: runAsk,
}

func runAsk(cmd *cobra.Command, args []string) error {
	question := args[0]

	flagsSet := 0
	if askFlags.project != "" {
		flagsSet++
	}
	if askFlags.repo != "" {
		flagsSet++
	}
	if askFlags.url != "" {
		flagsSet++
	}
	if askFlags.web {
		flagsSet++
	}

	if flagsSet > 1 {
		return fmt.Errorf("only one of --project, --repo, --url, or --web may be specified at a time")
	}

	usage.Log("ask", askLogTarget())

	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}

	maxSteps, maxTokens := resolveLimits(cmd, cfg, askFlags.maxSteps, askFlags.maxTokens)

	switch {
	case askFlags.project != "":
		return askProject(question, askFlags.project, cfg, maxSteps, maxTokens)
	case askFlags.repo != "":
		return askRepo(question, askFlags.repo, cfg, maxSteps, maxTokens)
	case askFlags.web:
		return askWeb(question, cfg, maxSteps, maxTokens)
	case askFlags.url != "":
		return askURL(question, askFlags.url, cfg, maxSteps, maxTokens)
	default:
		return askGeneral(question, cfg, maxSteps, maxTokens)
	}
}

// askProject asks about a registered ttal project.
func askProject(question, alias string, cfg *config.Config, maxSteps, maxTokens int) error {
	projectPath := project.ResolveProjectPath(alias)
	if projectPath == "" {
		return fmt.Errorf("project %q not found\n\nRun 'ttal project list' to see available projects", alias)
	}

	if _, err := os.Stat(projectPath); err != nil {
		return fmt.Errorf("project path %q does not exist on disk: %w", projectPath, err)
	}

	backend, err := resolveFetchBackend()
	if err != nil {
		return fmt.Errorf("resolve fetch backend: %w", err)
	}

	return runAskAgent(askOpts{
		question:     question,
		systemExtra:  strings.ReplaceAll(askProjectPrompt, "{projectPath}", projectPath),
		allowedPaths: []string{projectPath},
		toolNames:    askCodespaceTools,
		model:        cfg.AskModel(),
		fetchBackend: backend,
		maxSteps:     maxSteps,
		maxTokens:    maxTokens,
		emoji:        "🔭",
		label:        "ask --project " + alias,
	})
}

// askRepo asks about an open-source repository (auto-clone/pull).
func askRepo(question, repoRef string, cfg *config.Config, maxSteps, maxTokens int) error {
	referencesPath := cfg.AskReferencesPath()
	cloneURL, localPath, err := resolveRepoRef(repoRef, referencesPath)
	if err != nil {
		return err
	}

	if err := ensureRepo(cloneURL, localPath); err != nil {
		return err
	}

	backend, err := resolveFetchBackend()
	if err != nil {
		return fmt.Errorf("resolve fetch backend: %w", err)
	}

	return runAskAgent(askOpts{
		question:     question,
		systemExtra:  strings.ReplaceAll(askRepoPrompt, "{localPath}", localPath),
		allowedPaths: []string{localPath},
		toolNames:    askCodespaceTools,
		model:        cfg.AskModel(),
		fetchBackend: backend,
		maxSteps:     maxSteps,
		maxTokens:    maxTokens,
		emoji:        "🔭",
		label:        "ask --repo " + repoRef,
	})
}

// askURL asks about a web page using defuddle for pre-fetching.
func askURL(question, rawURL string, cfg *config.Config, maxSteps, maxTokens int) error {
	backend, err := resolveFetchBackend()
	if err != nil {
		return err
	}

	// Pre-warm the cache so the agent's read_url call is instant.
	fmt.Fprintf(os.Stderr, "Fetching %s...\n", rawURL)
	ctx := context.Background()
	if _, err := backend.Fetch(ctx, rawURL); err != nil {
		return fmt.Errorf("pre-fetch %s: %w", rawURL, err)
	}

	return runAskAgent(askOpts{
		question:     fmt.Sprintf("URL: %s\n\nQuestion: %s", rawURL, question),
		systemExtra:  strings.ReplaceAll(askURLPrompt, "{rawURL}", rawURL),
		allowedPaths: nil, // URL mode: no filesystem tools
		toolNames:    []string{"read_url", "search_web"},
		model:        cfg.AskModel(),
		fetchBackend: backend,
		maxSteps:     maxSteps,
		maxTokens:    maxTokens,
		emoji:        "🔭",
		label:        "ask --url",
	})
}

// askWeb searches the web to answer a question.
func askWeb(question string, cfg *config.Config, maxSteps, maxTokens int) error {
	backend, err := resolveFetchBackend()
	if err != nil {
		return err
	}

	prompt := strings.ReplaceAll(askWebPrompt, "{query}", question)

	return runAskAgent(askOpts{
		question:     question,
		systemExtra:  prompt,
		allowedPaths: nil,
		toolNames:    []string{"search_web", "read_url"},
		fetchBackend: backend,
		model:        cfg.AskModel(),
		maxSteps:     maxSteps,
		maxTokens:    maxTokens,
		emoji:        "🔭",
		label:        "ask --web",
	})
}

// askGeneral asks about the current working directory with both filesystem and web tools.
func askGeneral(question string, cfg *config.Config, maxSteps, maxTokens int) error {
	backend, err := resolveFetchBackend()
	if err != nil {
		return err
	}

	if !strings.Contains(askGeneralPrompt, "{cwd}") {
		return fmt.Errorf("general.md prompt is missing {cwd} placeholder")
	}

	cwd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("get working directory: %w", err)
	}

	prompt := strings.ReplaceAll(askGeneralPrompt, "{cwd}", cwd)

	return runAskAgent(askOpts{
		question:     question,
		systemExtra:  prompt,
		allowedPaths: []string{cwd},
		toolNames:    askCodespaceTools,
		fetchBackend: backend,
		model:        cfg.AskModel(),
		maxSteps:     maxSteps,
		maxTokens:    maxTokens,
		emoji:        "🔭",
		label:        "ask",
	})
}

// askOpts holds parameters for running the ask subagent.
type askOpts struct {
	question     string
	systemExtra  string               // mode-specific system prompt addition
	allowedPaths []string             // nil = no filesystem tools
	toolNames    []string             // which tools to enable
	model        string               // model string (e.g. "claude-sonnet-4-6")
	fetchBackend tools.ReadURLBackend // required: must be non-nil
	maxSteps     int
	maxTokens    int
	emoji        string // optional display emoji shown before output
	label        string // display name shown in header (defaults to "ask")
}

// runAskAgent builds and runs an agentloop for the ask command.
func runAskAgent(opts askOpts) error {
	label := opts.label
	if label == "" {
		label = "ask"
	}
	printAgentHeader(opts.emoji, label)

	provider, modelID, err := buildSubagentProvider(opts.model)
	if err != nil {
		return fmt.Errorf("build provider: %w", err)
	}

	selectedTools, err := buildToolSet(opts.toolNames, opts.allowedPaths, opts.fetchBackend)
	if err != nil {
		return err
	}

	var cwd string
	if len(opts.allowedPaths) > 0 {
		cwd, err = os.Getwd()
		if err != nil {
			return fmt.Errorf("get working directory: %w", err)
		}
	}

	allowedPaths := opts.allowedPaths
	systemPrompt, err := buildAgentSystemPrompt(cwd, allowedPaths, selectedTools, opts.systemExtra)
	if err != nil {
		return err
	}

	agentCfg := agentloop.Config{
		Provider:     provider,
		Model:        modelID,
		SystemPrompt: systemPrompt,
		Tools:        selectedTools,
		MaxSteps:     opts.maxSteps,
		MaxTokens:    opts.maxTokens,
		AllowedPaths: allowedPaths,
	}

	result, err := agentloop.Run(context.Background(), agentCfg, nil, opts.question, agentloop.Callbacks{
		OnDelta: func(text string) { fmt.Print(text) },
	})
	if err != nil {
		return fmt.Errorf("agent loop: %w", err)
	}

	if result.Response != "" && !strings.HasSuffix(result.Response, "\n") {
		fmt.Println()
	}

	return nil
}

// resolveRepoRef converts a repo reference to a clone URL and local path.
// Supports full URLs (https://github.com/org/repo) and shorthands (org/repo → GitHub).
// Shorthand must be exactly "org/repo" — bare names are rejected.
func resolveRepoRef(ref, referencesPath string) (cloneURL, localPath string, err error) {
	if strings.HasPrefix(ref, "https://") || strings.HasPrefix(ref, "http://") {
		u, parseErr := url.Parse(ref)
		if parseErr != nil {
			return "", "", fmt.Errorf("invalid URL: %w", parseErr)
		}
		repoPath := strings.TrimPrefix(u.Path, "/")
		repoPath = strings.TrimSuffix(repoPath, "/")
		repoPath = strings.TrimSuffix(repoPath, ".git")
		localPath = filepath.Join(referencesPath, u.Host, repoPath)
		cloneURL = ref
	} else {
		// Shorthand: must be "org/repo" format (exactly one slash)
		ref = strings.TrimSuffix(ref, "/")
		ref = strings.TrimSuffix(ref, ".git")
		parts := strings.Split(ref, "/")
		if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
			return "", "", fmt.Errorf(
				"invalid repo shorthand %q: expected \"org/repo\" (e.g. woodpecker-ci/woodpecker) or a full URL",
				ref,
			)
		}
		cloneURL = "https://github.com/" + ref
		localPath = filepath.Join(referencesPath, "github.com", ref)
	}
	return cloneURL, localPath, nil
}

const repoOpTimeout = 5 * time.Minute

// ensureRepo clones the repo if it doesn't exist, or pulls if it does.
func ensureRepo(cloneURL, localPath string) error {
	ctx, cancel := context.WithTimeout(context.Background(), repoOpTimeout)
	defer cancel()

	if dirExists(localPath) {
		fmt.Fprintf(os.Stderr, "Updating %s...\n", filepath.Base(localPath))
		cmd := exec.CommandContext(ctx, "git", "-C", localPath, "pull", "--ff-only")
		cmd.Stderr = os.Stderr
		if err := cmd.Run(); err != nil {
			return fmt.Errorf("git pull failed in %s: %w", localPath, err)
		}
		return nil
	}

	fmt.Fprintf(os.Stderr, "Cloning %s...\n", cloneURL)
	if err := os.MkdirAll(filepath.Dir(localPath), 0o755); err != nil {
		return fmt.Errorf("create parent directory: %w", err)
	}
	cmd := exec.CommandContext(ctx, "git", "clone", "--depth", "1", cloneURL, localPath)
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("git clone %s into %s: %w", cloneURL, localPath, err)
	}
	return nil
}

// dirExists reports whether path is an existing directory.
func dirExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && info.IsDir()
}

func init() {
	askCmd.Flags().StringVar(&askFlags.project, "project", "", "Ask about a registered ttal project by alias")
	askCmd.Flags().StringVar(&askFlags.repo, "repo", "", "Ask about an OSS repo (full URL or org/repo shorthand)")
	askCmd.Flags().StringVar(&askFlags.url, "url", "", "Ask about a web page (pre-fetched with defuddle)")
	askCmd.Flags().BoolVar(&askFlags.web, "web", false, "Search the web to answer the question")
	askCmd.Flags().IntVar(&askFlags.maxSteps, "max-steps", config.AskDefaultMaxSteps, "Maximum agent steps")               //nolint:lll
	askCmd.Flags().IntVar(&askFlags.maxTokens, "max-tokens", config.AskDefaultMaxTokens, "Maximum output tokens per step") //nolint:lll

	rootCmd.AddCommand(askCmd)
}

// askLogTarget returns the usage log target string based on active ask flags.
func askLogTarget() string {
	switch {
	case askFlags.project != "":
		return askFlags.project
	case askFlags.repo != "":
		return askFlags.repo
	case askFlags.url != "":
		return askFlags.url
	case askFlags.web:
		return "web"
	default:
		return "general"
	}
}
