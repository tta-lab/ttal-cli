package cmd

import (
	"context"
	_ "embed"
	"fmt"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"github.com/tta-lab/logos"
	"github.com/tta-lab/ttal-cli/internal/config"
	"github.com/tta-lab/ttal-cli/internal/project"
	"github.com/tta-lab/ttal-cli/internal/usage"
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

var askFlags struct {
	project   string
	repo      string
	url       string
	web       bool
	human     bool
	options   []string
	maxSteps  int
	maxTokens int
}

var askCmd = &cobra.Command{
	Use:   "ask <question>",
	Short: "Ask about code, repos, web pages, or the web using an AI agent",
	Long: `Ask a natural language question about a codebase, open-source repository, or web page.

With no flags, asks about the current directory with both filesystem and web access.
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
	if askFlags.human {
		flagsSet++
	}

	if flagsSet > 1 {
		return fmt.Errorf("only one of --project, --repo, --url, --web, or --human may be specified at a time\n\n  Example: ttal ask \"question\" --project ttal") //nolint:lll
	}

	if len(askFlags.options) > 0 && !askFlags.human {
		return fmt.Errorf("--option is only valid with --human\n\n  Example: ttal ask --human \"question\" --option \"yes\" --option \"no\"") //nolint:lll
	}

	if askFlags.human {
		return runAskHuman(cmd, args, askFlags.options)
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
	projectPath, err := project.GetProjectPath(alias)
	if err != nil {
		return err
	}

	if _, err := os.Stat(projectPath); err != nil {
		return fmt.Errorf("project path %q does not exist on disk: %w", projectPath, err)
	}

	return runAskAgent(askOpts{
		question:     question,
		systemExtra:  strings.ReplaceAll(askProjectPrompt, "{projectPath}", projectPath),
		workingDir:   projectPath,
		allowedPaths: []string{projectPath},
		network:      true,
		readFS:       true,
		model:        cfg.AskModel(),
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

	return runAskAgent(askOpts{
		question:     question,
		systemExtra:  strings.ReplaceAll(askRepoPrompt, "{localPath}", localPath),
		workingDir:   localPath,
		allowedPaths: []string{localPath},
		network:      true,
		readFS:       true,
		model:        cfg.AskModel(),
		maxSteps:     maxSteps,
		maxTokens:    maxTokens,
		emoji:        "🔭",
		label:        "ask --repo " + repoRef,
	})
}

// askURL asks about a web page using temenos for pre-fetching.
func askURL(question, rawURL string, cfg *config.Config, maxSteps, maxTokens int) error {
	return runAskAgent(askOpts{
		question:    fmt.Sprintf("URL: %s\n\nQuestion: %s", rawURL, question),
		systemExtra: strings.ReplaceAll(askURLPrompt, "{rawURL}", rawURL),
		network:     true,
		readFS:      false,
		preWarmURL:  rawURL,
		model:       cfg.AskModel(),
		maxSteps:    maxSteps,
		maxTokens:   maxTokens,
		emoji:       "🔭",
		label:       "ask --url",
	})
}

// askWeb searches the web to answer a question.
func askWeb(question string, cfg *config.Config, maxSteps, maxTokens int) error {
	return runAskAgent(askOpts{
		question:    question,
		systemExtra: strings.ReplaceAll(askWebPrompt, "{query}", question),
		network:     true,
		readFS:      false,
		model:       cfg.AskModel(),
		maxSteps:    maxSteps,
		maxTokens:   maxTokens,
		emoji:       "🔭",
		label:       "ask --web",
	})
}

// askGeneral asks about the current working directory with both filesystem and web tools.
func askGeneral(question string, cfg *config.Config, maxSteps, maxTokens int) error {
	if !strings.Contains(askGeneralPrompt, "{cwd}") {
		return fmt.Errorf("general.md prompt is missing {cwd} placeholder")
	}

	cwd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("get working directory: %w", err)
	}

	return runAskAgent(askOpts{
		question:     question,
		systemExtra:  strings.ReplaceAll(askGeneralPrompt, "{cwd}", cwd),
		workingDir:   cwd,
		allowedPaths: []string{cwd},
		network:      true,
		readFS:       true,
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
	systemExtra  string   // mode-specific system prompt addition
	workingDir   string   // working dir shown in system prompt (also used for PromptData)
	allowedPaths []string // nil = no filesystem access
	network      bool     // include read-url + search in prompt
	readFS       bool     // include rg + read-only filesystem guidance in prompt
	preWarmURL   string   // if set, pre-fetch via temenos before agent loop
	model        string   // model string (e.g. "claude-sonnet-4-6")
	maxSteps     int
	maxTokens    int
	emoji        string // optional display emoji shown before output
	label        string // display name shown in header (defaults to "ask")
}

// runAskAgent builds and runs a logos agent loop for the ask command.
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

	cwd := opts.workingDir

	promptData := logos.PromptData{
		WorkingDir: cwd,
		Platform:   runtime.GOOS,
		Date:       time.Now().Format("2006-01-02"),
		Network:    opts.network,
		ReadFS:     opts.readFS,
	}
	systemPrompt, err := logos.BuildSystemPrompt(promptData)
	if err != nil {
		return fmt.Errorf("build system prompt: %w", err)
	}
	if opts.systemExtra != "" {
		systemPrompt += "\n\n" + opts.systemExtra
	}

	tc, err := newTemenosClient(context.Background())
	if err != nil {
		return err
	}

	// Pre-warm URL cache if requested (used by --url mode).
	if opts.preWarmURL != "" {
		fmt.Fprintf(os.Stderr, "Fetching %s...\n", opts.preWarmURL)
		quotedURL := "'" + strings.ReplaceAll(opts.preWarmURL, "'", "'\\''") + "'"
		resp, err := tc.Run(context.Background(), logos.RunRequest{
			Command: "temenos read-url " + quotedURL,
		})
		if err != nil {
			return fmt.Errorf("pre-fetch %s: %w", opts.preWarmURL, err)
		}
		if resp.ExitCode != 0 {
			return fmt.Errorf("pre-fetch %s failed (exit %d): %s",
				opts.preWarmURL, resp.ExitCode, strings.TrimSpace(resp.Stderr))
		}
	}

	var allowedPaths []logos.AllowedPath
	for _, p := range opts.allowedPaths {
		allowedPaths = append(allowedPaths, logos.AllowedPath{Path: p, ReadOnly: true})
	}

	cfg := logos.Config{
		Provider:     provider,
		Model:        modelID,
		SystemPrompt: systemPrompt,
		MaxSteps:     opts.maxSteps,
		MaxTokens:    opts.maxTokens,
		Temenos:      tc,
		AllowedPaths: allowedPaths,
	}

	result, err := logos.Run(context.Background(), cfg, nil, opts.question, logos.Callbacks{
		OnDelta:        func(text string) { fmt.Print(text) },
		OnCommandStart: renderCommandStart,
		OnCommandResult: func(command, output string, exitCode int) {
			renderCommandResult(output, exitCode)
		},
		OnRetry: renderRetry,
	})
	return flushAgentResult(result, err)
}

// resolveRepoRef converts a repo reference to a clone URL and local path.
// Supports full URLs, "org/repo" shorthands (→ GitHub), and bare repo names
// for repos that are already cloned locally in the references directory.
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
	} else if strings.Contains(ref, "/") {
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
	} else {
		// Bare name: scan already-cloned repos for a match.
		// Only works for repos that are already cloned locally.
		localPath, err = findClonedRepo(ref, referencesPath)
		if err != nil {
			return "", "", err
		}
		// Derive cloneURL from local path for ensureRepo's git-pull path.
		// Note: bare-name repos are always already cloned, so ensureRepo will
		// only use this for "git pull", never "git clone".
		rel, relErr := filepath.Rel(referencesPath, localPath)
		if relErr != nil {
			return "", "", fmt.Errorf("computing repo clone URL from %s: %w", localPath, relErr)
		}
		cloneURL = "https://" + rel
	}
	return cloneURL, localPath, nil
}

// scanHostDir scans host/org/repo under hostPath, collecting paths where the
// final directory component equals name.
func scanHostDir(name, hostPath string) []string {
	var matches []string
	orgs, err := os.ReadDir(hostPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "warning: skipping %s: %v\n", hostPath, err)
		return nil
	}
	for _, org := range orgs {
		if !org.IsDir() {
			continue
		}
		orgPath := filepath.Join(hostPath, org.Name())
		repos, err := os.ReadDir(orgPath)
		if err != nil {
			fmt.Fprintf(os.Stderr, "warning: skipping %s: %v\n", orgPath, err)
			continue
		}
		for _, repo := range repos {
			if repo.IsDir() && repo.Name() == name {
				matches = append(matches, filepath.Join(orgPath, repo.Name()))
			}
		}
	}
	return matches
}

// findClonedRepo scans the references directory for an already-cloned repo
// matching the bare name (case-sensitive). Returns the local path if exactly
// one match is found. Errors with disambiguation list on multiple matches.
func findClonedRepo(name, referencesPath string) (string, error) {
	var matches []string

	hosts, err := os.ReadDir(referencesPath)
	if err != nil {
		if os.IsNotExist(err) {
			return "", fmt.Errorf(
				"repo %q not found as org/repo; references directory does not exist at %s",
				name, referencesPath,
			)
		}
		return "", fmt.Errorf(
			"repo %q not found as org/repo; could not read references directory %s: %w",
			name, referencesPath, err,
		)
	}

	for _, host := range hosts {
		if !host.IsDir() {
			continue
		}
		hostPath := filepath.Join(referencesPath, host.Name())
		matches = append(matches, scanHostDir(name, hostPath)...)
	}

	switch len(matches) {
	case 0:
		return "", fmt.Errorf(
			"repo %q not found locally; use org/repo format (e.g. charmbracelet/%s) to clone it",
			name, name,
		)
	case 1:
		return matches[0], nil
	default:
		var options []string
		for _, m := range matches {
			rel, relErr := filepath.Rel(referencesPath, m)
			if relErr != nil {
				options = append(options, m) // fallback to absolute path
				continue
			}
			parts := strings.SplitN(rel, string(filepath.Separator), 2)
			if len(parts) == 2 {
				options = append(options, parts[1])
			} else {
				options = append(options, rel)
			}
		}
		return "", fmt.Errorf(
			"ambiguous repo name %q matches multiple repos:\n  %s\n\nSpecify org/repo to disambiguate",
			name, strings.Join(options, "\n  "),
		)
	}
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
	askCmd.Flags().BoolVar(&askFlags.human, "human", false, "Ask a human via Telegram and block until answered")
	askCmd.Flags().StringArrayVar(&askFlags.options, "option", nil, "Add an option button (repeatable, only valid with --human)") //nolint:lll
	askCmd.Flags().IntVar(&askFlags.maxSteps, "max-steps", config.AskDefaultMaxSteps, "Maximum agent steps")                      //nolint:lll
	askCmd.Flags().IntVar(&askFlags.maxTokens, "max-tokens", config.AskDefaultMaxTokens, "Maximum output tokens per step")        //nolint:lll

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
	case askFlags.human:
		return "human"
	default:
		return "general"
	}
}
