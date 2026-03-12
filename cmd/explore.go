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

//go:embed explore_prompts/project.md
var exploreProjectPrompt string

//go:embed explore_prompts/repo.md
var exploreRepoPrompt string

//go:embed explore_prompts/url.md
var exploreURLPrompt string

//go:embed explore_prompts/web.md
var exploreWebPrompt string

var exploreFlags struct {
	project   string
	repo      string
	url       string
	web       bool
	maxSteps  int
	maxTokens int
}

var exploreCmd = &cobra.Command{
	Use:   "explore <question>",
	Short: "Explore a project, repo, web page, or the web with an AI agent",
	Long: `Explore a codebase, open-source repository, web page, or the web by asking a natural language question.

Exactly one source flag must be specified:
  --project <alias>      Explore a registered ttal project
  --repo <url|org/repo>  Explore a GitHub repo (auto-clone/pull)
  --url <url>            Explore a web page (pre-fetched with defuddle)
  --web                  Search the web to answer the question

Examples:
  ttal explore "how does routing work?" --project ttal-cli
  ttal explore "how does pipeline syntax work?" --repo woodpecker-ci/woodpecker
  ttal explore "what API endpoints are available?" --url https://docs.example.com
  ttal explore "what is the latest Go generics syntax?" --web`,
	Args: cobra.ExactArgs(1),
	RunE: runExplore,
}

func runExplore(cmd *cobra.Command, args []string) error {
	question := args[0]

	flagsSet := 0
	if exploreFlags.project != "" {
		flagsSet++
	}
	if exploreFlags.repo != "" {
		flagsSet++
	}
	if exploreFlags.url != "" {
		flagsSet++
	}
	if exploreFlags.web {
		flagsSet++
	}

	if flagsSet == 0 {
		return fmt.Errorf("one of --project, --repo, --url, or --web is required\n\nRun 'ttal explore --help' for usage")
	}
	if flagsSet > 1 {
		return fmt.Errorf("only one of --project, --repo, --url, or --web may be specified at a time")
	}

	usage.Log("explore", exploreLogTarget())

	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}

	maxSteps, maxTokens := resolveLimits(cmd, cfg, exploreFlags.maxSteps, exploreFlags.maxTokens)

	switch {
	case exploreFlags.project != "":
		return exploreProject(question, exploreFlags.project, cfg, maxSteps, maxTokens)
	case exploreFlags.repo != "":
		return exploreRepo(question, exploreFlags.repo, cfg, maxSteps, maxTokens)
	case exploreFlags.web:
		return exploreWeb(question, cfg, maxSteps, maxTokens)
	case exploreFlags.url != "":
		return exploreURL(question, exploreFlags.url, cfg, maxSteps, maxTokens)
	default:
		return fmt.Errorf("internal: unhandled explore mode")
	}
}

// exploreProject explores a registered ttal project.
func exploreProject(question, alias string, cfg *config.Config, maxSteps, maxTokens int) error {
	projectPath := project.ResolveProjectPath(alias)
	if projectPath == "" {
		return fmt.Errorf("project %q not found\n\nRun 'ttal project list' to see available projects", alias)
	}

	if _, err := os.Stat(projectPath); err != nil {
		return fmt.Errorf("project path %q does not exist on disk: %w", projectPath, err)
	}

	return runExploreAgent(exploreOpts{
		question:     question,
		systemExtra:  strings.ReplaceAll(exploreProjectPrompt, "{projectPath}", projectPath),
		allowedPaths: []string{projectPath},
		toolNames:    []string{"bash", "read", "read_md", "glob", "grep"},
		model:        cfg.ExploreModel(),
		fetchBackend: tools.NewDefuddleCLIBackend(), // placeholder: read_url not in toolNames
		maxSteps:     maxSteps,
		maxTokens:    maxTokens,
	})
}

// exploreRepo explores an open-source repository (auto-clone/pull).
func exploreRepo(question, repoRef string, cfg *config.Config, maxSteps, maxTokens int) error {
	referencesPath := cfg.ExploreReferencesPath()
	cloneURL, localPath, err := resolveRepoRef(repoRef, referencesPath)
	if err != nil {
		return err
	}

	if err := ensureRepo(cloneURL, localPath); err != nil {
		return err
	}

	return runExploreAgent(exploreOpts{
		question:     question,
		systemExtra:  strings.ReplaceAll(exploreRepoPrompt, "{localPath}", localPath),
		allowedPaths: []string{localPath},
		toolNames:    []string{"bash", "read", "read_md", "glob", "grep"},
		model:        cfg.ExploreModel(),
		fetchBackend: tools.NewDefuddleCLIBackend(), // placeholder: read_url not in toolNames
		maxSteps:     maxSteps,
		maxTokens:    maxTokens,
	})
}

// exploreURL explores a web page using defuddle for pre-fetching.
func exploreURL(question, rawURL string, cfg *config.Config, maxSteps, maxTokens int) error {
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

	return runExploreAgent(exploreOpts{
		question:     fmt.Sprintf("URL: %s\n\nQuestion: %s", rawURL, question),
		systemExtra:  strings.ReplaceAll(exploreURLPrompt, "{rawURL}", rawURL),
		allowedPaths: nil, // URL mode: no filesystem tools
		toolNames:    []string{"read_url", "search_web"},
		model:        cfg.ExploreModel(),
		fetchBackend: backend,
		maxSteps:     maxSteps,
		maxTokens:    maxTokens,
	})
}

// exploreWeb searches the web to answer a question.
func exploreWeb(question string, cfg *config.Config, maxSteps, maxTokens int) error {
	backend, err := resolveFetchBackend()
	if err != nil {
		return err
	}

	prompt := strings.ReplaceAll(exploreWebPrompt, "{query}", question)

	return runExploreAgent(exploreOpts{
		question:     question,
		systemExtra:  prompt,
		allowedPaths: nil,
		toolNames:    []string{"search_web", "read_url"},
		fetchBackend: backend,
		model:        cfg.ExploreModel(),
		maxSteps:     maxSteps,
		maxTokens:    maxTokens,
	})
}

// exploreOpts holds parameters for running the explore subagent.
type exploreOpts struct {
	question     string
	systemExtra  string               // mode-specific system prompt addition
	allowedPaths []string             // nil = no filesystem tools
	toolNames    []string             // which tools to enable
	model        string               // model string (e.g. "claude-sonnet-4-6")
	fetchBackend tools.ReadURLBackend // required: must be non-nil
	maxSteps     int
	maxTokens    int
}

// runExploreAgent builds and runs an agentloop for the explore command.
func runExploreAgent(opts exploreOpts) error {
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

	result, err := agentloop.Run(context.Background(), agentCfg, nil, opts.question, func(text string) {
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
	exploreCmd.Flags().StringVar(&exploreFlags.project, "project", "", "Explore a registered ttal project by alias")
	exploreCmd.Flags().StringVar(&exploreFlags.repo, "repo", "", "Explore an OSS repo (full URL or org/repo shorthand)")
	exploreCmd.Flags().StringVar(&exploreFlags.url, "url", "", "Explore a web page (pre-fetched with defuddle)")
	exploreCmd.Flags().BoolVar(&exploreFlags.web, "web", false, "Search the web to answer the question")
	exploreCmd.Flags().IntVar(&exploreFlags.maxSteps, "max-steps", config.ExploreDefaultMaxSteps, "Maximum agent steps")               //nolint:lll
	exploreCmd.Flags().IntVar(&exploreFlags.maxTokens, "max-tokens", config.ExploreDefaultMaxTokens, "Maximum output tokens per step") //nolint:lll

	rootCmd.AddCommand(exploreCmd)
}

// exploreLogTarget returns the usage log target string based on active explore flags.
func exploreLogTarget() string {
	switch {
	case exploreFlags.project != "":
		return exploreFlags.project
	case exploreFlags.repo != "":
		return exploreFlags.repo
	case exploreFlags.url != "":
		return exploreFlags.url
	case exploreFlags.web:
		return "web"
	default:
		return "general"
	}
}
