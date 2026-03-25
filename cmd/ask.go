package cmd

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"github.com/tta-lab/logos"
	"github.com/tta-lab/ttal-cli/internal/ask"
	"github.com/tta-lab/ttal-cli/internal/config"
	"github.com/tta-lab/ttal-cli/internal/project"
	"github.com/tta-lab/ttal-cli/internal/usage"
)

var askFlags struct {
	project   string
	repo      string
	url       string
	web       bool
	human     bool
	save      bool
	quiet     bool
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
	PreRunE: func(cmd *cobra.Command, args []string) error {
		// ttal ask needs MINIMAX_API_KEY and BRAVE_API_KEY for subagent.
		// Load .env as fallback for tokens not already in the environment.
		if err := config.InjectDotEnvFallback(); err != nil {
			fmt.Fprintf(os.Stderr, "warning: could not load .env: %v\n", err)
		}
		return nil
	},
	RunE: runAsk,
}

// countAskSourceFlags returns the number of mutually exclusive source flags set.
func countAskSourceFlags() int {
	count := 0
	if askFlags.project != "" {
		count++
	}
	if askFlags.repo != "" {
		count++
	}
	if askFlags.url != "" {
		count++
	}
	if askFlags.web {
		count++
	}
	if askFlags.human {
		count++
	}
	return count
}

func runAsk(cmd *cobra.Command, args []string) error {
	question := args[0]

	if countAskSourceFlags() > 1 {
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

	params := ask.ModeParams{
		WorkingDir:  projectPath,
		ProjectPath: projectPath,
	}
	systemPrompt, _, err := ask.BuildSystemPromptForMode(ask.ModeProject, params)
	if err != nil {
		return fmt.Errorf("build system prompt: %w", err)
	}

	return runAskAgent(askOpts{
		question:     question,
		systemPrompt: systemPrompt,
		allowedPaths: []string{projectPath},
		model:        cfg.AskModel(),
		maxSteps:     maxSteps,
		maxTokens:    maxTokens,
		emoji:        "🔭",
		label:        "ask --project " + alias,
		save:         askFlags.save,
		quiet:        askFlags.quiet || cfg.AskOutput() == config.AskOutputQuiet,
	})
}

// askRepo asks about an open-source repository (auto-clone/pull).
func askRepo(question, repoRef string, cfg *config.Config, maxSteps, maxTokens int) error {
	referencesPath := cfg.AskReferencesPath()
	cloneURL, localPath, err := ask.ResolveRepoRef(repoRef, referencesPath)
	if err != nil {
		return err
	}

	if err := ask.EnsureRepo(cloneURL, localPath); err != nil {
		return err
	}

	params := ask.ModeParams{
		WorkingDir:    localPath,
		RepoLocalPath: localPath,
	}
	systemPrompt, _, err := ask.BuildSystemPromptForMode(ask.ModeRepo, params)
	if err != nil {
		return fmt.Errorf("build system prompt: %w", err)
	}

	return runAskAgent(askOpts{
		question:     question,
		systemPrompt: systemPrompt,
		allowedPaths: []string{localPath},
		model:        cfg.AskModel(),
		maxSteps:     maxSteps,
		maxTokens:    maxTokens,
		emoji:        "🔭",
		label:        "ask --repo " + repoRef,
		save:         askFlags.save,
		quiet:        askFlags.quiet || cfg.AskOutput() == config.AskOutputQuiet,
	})
}

// askURL asks about a web page using url for pre-fetching.
func askURL(question, rawURL string, cfg *config.Config, maxSteps, maxTokens int) error {
	params := ask.ModeParams{
		RawURL: rawURL,
	}
	systemPrompt, _, err := ask.BuildSystemPromptForMode(ask.ModeURL, params)
	if err != nil {
		return fmt.Errorf("build system prompt: %w", err)
	}

	return runAskAgent(askOpts{
		question:     fmt.Sprintf("URL: %s\n\nQuestion: %s", rawURL, question),
		systemPrompt: systemPrompt,
		preWarmURL:   rawURL,
		model:        cfg.AskModel(),
		maxSteps:     maxSteps,
		maxTokens:    maxTokens,
		emoji:        "🔭",
		label:        "ask --url",
		save:         askFlags.save,
		quiet:        askFlags.quiet || cfg.AskOutput() == config.AskOutputQuiet,
	})
}

// askWeb searches the web to answer a question.
func askWeb(question string, cfg *config.Config, maxSteps, maxTokens int) error {
	params := ask.ModeParams{
		Question: question,
	}
	systemPrompt, _, err := ask.BuildSystemPromptForMode(ask.ModeWeb, params)
	if err != nil {
		return fmt.Errorf("build system prompt: %w", err)
	}

	return runAskAgent(askOpts{
		question:     question,
		systemPrompt: systemPrompt,
		model:        cfg.AskModel(),
		maxSteps:     maxSteps,
		maxTokens:    maxTokens,
		emoji:        "🔭",
		label:        "ask --web",
		save:         askFlags.save,
		quiet:        askFlags.quiet || cfg.AskOutput() == config.AskOutputQuiet,
	})
}

// askGeneral asks about the current working directory with both filesystem and web tools.
func askGeneral(question string, cfg *config.Config, maxSteps, maxTokens int) error {
	cwd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("get working directory: %w", err)
	}

	params := ask.ModeParams{
		WorkingDir: cwd,
		Question:   question,
	}
	systemPrompt, _, err := ask.BuildSystemPromptForMode(ask.ModeGeneral, params)
	if err != nil {
		return fmt.Errorf("build system prompt: %w", err)
	}

	return runAskAgent(askOpts{
		question:     question,
		systemPrompt: systemPrompt,
		allowedPaths: []string{cwd},
		model:        cfg.AskModel(),
		maxSteps:     maxSteps,
		maxTokens:    maxTokens,
		emoji:        "🔭",
		label:        "ask",
		save:         askFlags.save,
		quiet:        askFlags.quiet || cfg.AskOutput() == config.AskOutputQuiet,
	})
}

// askOpts holds parameters for running the ask subagent.
type askOpts struct {
	question     string
	systemPrompt string   // full system prompt (pre-built by mode functions)
	allowedPaths []string // nil = no filesystem access
	preWarmURL   string   // if set, pre-fetch via url before agent loop
	model        string   // model string (e.g. "claude-sonnet-4-6")
	maxSteps     int
	maxTokens    int
	emoji        string // optional display emoji shown before output
	label        string // display name shown in header (defaults to "ask")
	save         bool   // if true, pipe final answer to flicknote add
	quiet        bool   // if true, suppress streaming and print result.Response after completion
}

// runAskAgent builds and runs a logos agent loop for the ask command.
func runAskAgent(opts askOpts) error {
	label := opts.label
	if label == "" {
		label = "ask"
	}
	printAgentHeader(opts.emoji, label)

	provider, modelID, err := ask.BuildProvider(opts.model)
	if err != nil {
		return fmt.Errorf("build provider: %w", err)
	}

	tc, err := ask.NewTemenosClient(context.Background())
	if err != nil {
		return err
	}

	// Pre-warm URL cache if requested (used by --url mode).
	if opts.preWarmURL != "" {
		fmt.Fprintf(os.Stderr, "Fetching %s...\n", opts.preWarmURL)
		quotedURL := "'" + strings.ReplaceAll(opts.preWarmURL, "'", "'\\''") + "'"
		resp, err := tc.Run(context.Background(), logos.RunRequest{
			Command: "url " + quotedURL,
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
		SystemPrompt: opts.systemPrompt,
		MaxSteps:     opts.maxSteps,
		MaxTokens:    opts.maxTokens,
		Temenos:      tc,
		AllowedPaths: allowedPaths,
	}

	callbacks, sp := buildAskCallbacks(opts.quiet)
	defer sp.Stop()
	result, err := logos.Run(context.Background(), cfg, nil, opts.question, callbacks)

	if opts.quiet && err == nil {
		printQuietResponse(result)
	}

	flushErr := flushAgentResult(result, err)

	// Only save on success — if agent hit max-steps or errored, skip save.
	if opts.save && flushErr == nil {
		if saveErr := saveAskResult(result); saveErr != nil {
			fmt.Fprintf(os.Stderr, "warning: failed to save to flicknote: %v\n", saveErr)
		}
	}

	return flushErr
}

// saveAskResult pipes the agent's final answer to flicknote add.
func saveAskResult(result *logos.RunResult) error {
	if result == nil {
		return nil
	}
	var finalAnswer string
	for i := len(result.Steps) - 1; i >= 0; i-- {
		if result.Steps[i].Role == logos.StepRoleAssistant {
			finalAnswer = result.Steps[i].Content
			break
		}
	}
	if finalAnswer == "" {
		return fmt.Errorf("no assistant content found in result, nothing saved")
	}

	cmd := exec.Command("flicknote", "add")
	cmd.Stdin = strings.NewReader(finalAnswer)
	cmd.Stderr = os.Stderr
	out, err := cmd.Output()
	if err != nil {
		if errors.Is(err, exec.ErrNotFound) {
			return fmt.Errorf("flicknote not found in PATH — install it first: https://github.com/tta-lab/flicknote-cli")
		}
		return err
	}
	fmt.Fprintf(os.Stdout, "%s", string(out))
	return nil
}

func init() {
	askCmd.Flags().StringVar(&askFlags.project, "project", "", "Ask about a registered ttal project by alias")
	askCmd.Flags().StringVar(&askFlags.repo, "repo", "", "Ask about an OSS repo (full URL or org/repo shorthand)")
	askCmd.Flags().StringVar(&askFlags.url, "url", "", "Ask about a web page (pre-fetched with defuddle)")
	askCmd.Flags().BoolVar(&askFlags.web, "web", false, "Search the web to answer the question")
	askCmd.Flags().BoolVar(&askFlags.human, "human", false, "Ask a human via Telegram and block until answered")
	askCmd.Flags().BoolVar(&askFlags.save, "save", false, "Save the final answer to flicknote (best-effort; failures are logged to stderr)") //nolint:lll
	askCmd.Flags().BoolVar(&askFlags.quiet, "quiet", false, "Show only assistant text (no streaming, no command traces)")
	askCmd.Flags().StringArrayVar(&askFlags.options, "option", nil, "Add an option button (repeatable, only valid with --human)") //nolint:lll
	askCmd.Flags().IntVar(&askFlags.maxSteps, "max-steps", config.AskDefaultMaxSteps, "Maximum agent steps")                      //nolint:lll
	askCmd.Flags().IntVar(&askFlags.maxTokens, "max-tokens", config.AskDefaultMaxTokens, "Maximum output tokens per step")        //nolint:lll

	rootCmd.AddCommand(askCmd)
}

// buildAskCallbacks returns the logos callbacks and an optional spinner for the given mode.
// In quiet mode all callbacks are nil and a spinner is started on TTY stderr.
// In verbose mode the full streaming callbacks are wired and spinner is nil.
func buildAskCallbacks(quiet bool) (logos.Callbacks, *spinner) {
	if quiet {
		var sp *spinner
		if isTerminal(os.Stderr) {
			sp = startSpinner()
		}
		return logos.Callbacks{}, sp
	}
	return logos.Callbacks{
		OnDelta:        renderDelta,
		OnCommandStart: renderCommandStart,
		OnCommandResult: func(command, output string, exitCode int) {
			renderCommandResult(output, exitCode)
		},
		OnRetry: renderRetry,
	}, nil
}

// printQuietResponse prints result.Response to stdout when it contains text.
// Used by quiet mode to emit accumulated assistant prose after the run completes.
func printQuietResponse(result *logos.RunResult) {
	if result != nil && result.Response != "" {
		fmt.Print(result.Response)
	}
}

// isTerminal reports whether f is connected to a terminal.
func isTerminal(f *os.File) bool {
	fi, err := f.Stat()
	if err != nil {
		return false
	}
	return (fi.Mode() & os.ModeCharDevice) != 0
}

// spinner displays an animated spinner on stderr while the agent runs in quiet mode.
type spinner struct {
	done chan struct{}
}

func startSpinner() *spinner {
	s := &spinner{done: make(chan struct{})}
	go func() {
		frames := []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"}
		i := 0
		ticker := time.NewTicker(80 * time.Millisecond)
		defer ticker.Stop()
		for {
			select {
			case <-s.done:
				fmt.Fprintf(os.Stderr, "\r\033[K") // clear spinner line
				return
			case <-ticker.C:
				fmt.Fprintf(os.Stderr, "\r%s thinking...", frames[i%len(frames)])
				i++
			}
		}
	}()
	return s
}

func (s *spinner) Stop() {
	if s == nil {
		return
	}
	select {
	case <-s.done:
		// already stopped
	default:
		close(s.done)
	}
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
