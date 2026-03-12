package cmd

import (
	"context"
	"fmt"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"github.com/tta-lab/ttal-cli/internal/config"
	"github.com/tta-lab/ttal-cli/internal/project"
	"github.com/tta-lab/ttal-cli/pkg/agentloop"
	"github.com/tta-lab/ttal-cli/pkg/agentloop/tools"
)

var exploreFlags struct {
	project   string
	repo      string
	url       string
	maxSteps  int
	maxTokens int
}

var exploreCmd = &cobra.Command{
	Use:   "explore <question>",
	Short: "Explore a project, repo, or web page with an AI agent",
	Long: `Explore a codebase, open-source repository, or web page by asking a natural language question.

Exactly one source flag must be specified:
  --project <alias>    Explore a registered ttal project
  --repo <url|org/repo> Explore a GitHub repo (auto-clone/pull)
  --url <url>          Explore a web page (pre-fetched with defuddle)

Examples:
  ttal explore "how does routing work?" --project ttal-cli
  ttal explore "how does pipeline syntax work?" --repo woodpecker-ci/woodpecker
  ttal explore "what API endpoints are available?" --url https://docs.example.com`,
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

	if flagsSet == 0 {
		return fmt.Errorf("one of --project, --repo, or --url is required\n\nRun 'ttal explore --help' for usage")
	}
	if flagsSet > 1 {
		return fmt.Errorf("only one of --project, --repo, or --url may be specified at a time")
	}

	switch {
	case exploreFlags.project != "":
		return exploreProject(question, exploreFlags.project)
	case exploreFlags.repo != "":
		return exploreRepo(question, exploreFlags.repo)
	default:
		return exploreURL(question, exploreFlags.url)
	}
}

// exploreProject explores a registered ttal project.
func exploreProject(question, alias string) error {
	projectPath := project.ResolveProjectPath(alias)
	if projectPath == "" {
		return fmt.Errorf("project %q not found\n\nRun 'ttal project list' to see available projects", alias)
	}

	if _, err := os.Stat(projectPath); err != nil {
		return fmt.Errorf("project path %q does not exist on disk: %w", projectPath, err)
	}

	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}

	return runExploreAgent(exploreOpts{
		question:     question,
		systemExtra:  fmt.Sprintf("You are exploring the codebase at %s. Answer the user's question.", projectPath),
		allowedPaths: []string{projectPath},
		toolNames:    []string{"bash", "read", "read_md", "glob", "grep"},
		model:        cfg.ExploreModel(),
		maxSteps:     exploreFlags.maxSteps,
		maxTokens:    exploreFlags.maxTokens,
	})
}

// exploreRepo explores an open-source repository (auto-clone/pull).
func exploreRepo(question, repoRef string) error {
	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}

	referencesPath := cfg.ExploreReferencesPath()
	cloneURL, localPath, err := resolveRepoRef(repoRef, referencesPath)
	if err != nil {
		return err
	}

	if err := ensureRepo(cloneURL, localPath); err != nil {
		return err
	}

	return runExploreAgent(exploreOpts{
		question: question,
		systemExtra: fmt.Sprintf( //nolint:lll
			"You are exploring the repository at %s (cloned from %s). Answer the user's question.",
			localPath, cloneURL,
		),
		allowedPaths: []string{localPath},
		toolNames:    []string{"bash", "read", "read_md", "glob", "grep"},
		model:        cfg.ExploreModel(),
		maxSteps:     exploreFlags.maxSteps,
		maxTokens:    exploreFlags.maxTokens,
	})
}

// exploreURL explores a web page using defuddle for pre-fetching.
func exploreURL(question, rawURL string) error {
	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}

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
		question: question,
		systemExtra: fmt.Sprintf( //nolint:lll
			"You are analyzing web content from %s. Use the read_url tool to access the content. Answer the user's question.",
			rawURL,
		),
		allowedPaths: nil, // no filesystem access for URL mode
		toolNames:    []string{"read_url", "search_web", "bash"},
		model:        cfg.ExploreModel(),
		fetchBackend: backend, // pass pre-resolved backend to avoid double init
		maxSteps:     exploreFlags.maxSteps,
		maxTokens:    exploreFlags.maxTokens,
	})
}

// exploreOpts holds parameters for running the explore subagent.
type exploreOpts struct {
	question     string
	systemExtra  string               // mode-specific system prompt addition
	allowedPaths []string             // nil = no filesystem tools
	toolNames    []string             // which tools to enable
	model        string               // model string (e.g. "claude-sonnet-4-6")
	fetchBackend tools.ReadURLBackend // optional: pre-resolved backend (nil = resolve fresh)
	maxSteps     int
	maxTokens    int
}

// runExploreAgent builds and runs an agentloop for the explore command.
func runExploreAgent(opts exploreOpts) error {
	provider, modelID, err := buildSubagentProvider(opts.model)
	if err != nil {
		return fmt.Errorf("build provider: %w", err)
	}

	backend := opts.fetchBackend
	if backend == nil {
		backend, err = resolveFetchBackend()
		if err != nil {
			return err
		}
	}

	selectedTools, err := buildToolSet(opts.toolNames, opts.allowedPaths, backend)
	if err != nil {
		return err
	}

	cwd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("get working directory: %w", err)
	}

	allowedPaths := opts.allowedPaths
	if len(allowedPaths) == 0 {
		allowedPaths = []string{cwd}
	}

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
		return fmt.Errorf("build system prompt: %w", err)
	}

	systemPrompt := base
	if opts.systemExtra != "" {
		systemPrompt = base + "\n\n" + opts.systemExtra
	}

	cfg := agentloop.Config{
		Provider:     provider,
		Model:        modelID,
		SystemPrompt: systemPrompt,
		Tools:        selectedTools,
		MaxSteps:     opts.maxSteps,
		MaxTokens:    opts.maxTokens,
		AllowedPaths: allowedPaths,
	}

	result, err := agentloop.Run(context.Background(), cfg, nil, opts.question, func(text string) {
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
		// Shorthand: "org/repo" → GitHub
		ref = strings.TrimSuffix(ref, "/")
		ref = strings.TrimSuffix(ref, ".git")
		cloneURL = "https://github.com/" + ref
		localPath = filepath.Join(referencesPath, "github.com", ref)
	}
	return cloneURL, localPath, nil
}

// ensureRepo clones the repo if it doesn't exist, or pulls if it does.
func ensureRepo(cloneURL, localPath string) error {
	if dirExists(localPath) {
		fmt.Fprintf(os.Stderr, "Updating %s...\n", filepath.Base(localPath))
		cmd := exec.Command("git", "-C", localPath, "pull", "--ff-only")
		cmd.Stderr = os.Stderr
		if err := cmd.Run(); err != nil {
			return fmt.Errorf("git pull failed in %s: %w (try deleting and re-cloning)", localPath, err)
		}
		return nil
	}

	fmt.Fprintf(os.Stderr, "Cloning %s...\n", cloneURL)
	if err := os.MkdirAll(filepath.Dir(localPath), 0o755); err != nil {
		return fmt.Errorf("create parent directory: %w", err)
	}
	cmd := exec.Command("git", "clone", "--depth", "1", cloneURL, localPath)
	cmd.Stderr = os.Stderr
	return cmd.Run()
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
	exploreCmd.Flags().IntVar(&exploreFlags.maxSteps, "max-steps", 20, "Maximum agent steps")
	exploreCmd.Flags().IntVar(&exploreFlags.maxTokens, "max-tokens", 4096, "Maximum output tokens per step")

	rootCmd.AddCommand(exploreCmd)
}
