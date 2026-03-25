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
	"github.com/tta-lab/ttal-cli/internal/ask"
	"github.com/tta-lab/ttal-cli/internal/config"
	"github.com/tta-lab/ttal-cli/internal/daemon"
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
	return runAskAgent(askOpts{
		question:  question,
		mode:      ask.ModeProject,
		project:   alias,
		maxSteps:  maxSteps,
		maxTokens: maxTokens,
		emoji:     "🔭",
		label:     "ask --project " + alias,
		save:      askFlags.save,
		quiet:     askFlags.quiet || cfg.AskOutput() == config.AskOutputQuiet,
	})
}

// askRepo asks about an open-source repository (auto-clone/pull).
func askRepo(question, repoRef string, cfg *config.Config, maxSteps, maxTokens int) error {
	return runAskAgent(askOpts{
		question:  question,
		mode:      ask.ModeRepo,
		repo:      repoRef,
		maxSteps:  maxSteps,
		maxTokens: maxTokens,
		emoji:     "🔭",
		label:     "ask --repo " + repoRef,
		save:      askFlags.save,
		quiet:     askFlags.quiet || cfg.AskOutput() == config.AskOutputQuiet,
	})
}

// askURL asks about a web page using url for pre-fetching.
func askURL(question, rawURL string, cfg *config.Config, maxSteps, maxTokens int) error {
	return runAskAgent(askOpts{
		question:  question,
		mode:      ask.ModeURL,
		rawURL:    rawURL,
		maxSteps:  maxSteps,
		maxTokens: maxTokens,
		emoji:     "🔭",
		label:     "ask --url",
		save:      askFlags.save,
		quiet:     askFlags.quiet || cfg.AskOutput() == config.AskOutputQuiet,
	})
}

// askWeb searches the web to answer a question.
func askWeb(question string, cfg *config.Config, maxSteps, maxTokens int) error {
	return runAskAgent(askOpts{
		question:  question,
		mode:      ask.ModeWeb,
		maxSteps:  maxSteps,
		maxTokens: maxTokens,
		emoji:     "🔭",
		label:     "ask --web",
		save:      askFlags.save,
		quiet:     askFlags.quiet || cfg.AskOutput() == config.AskOutputQuiet,
	})
}

// askGeneral asks about the current working directory with both filesystem and web tools.
func askGeneral(question string, cfg *config.Config, maxSteps, maxTokens int) error {
	cwd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("get working directory: %w", err)
	}
	return runAskAgent(askOpts{
		question:   question,
		mode:       ask.ModeGeneral,
		workingDir: cwd,
		maxSteps:   maxSteps,
		maxTokens:  maxTokens,
		emoji:      "🔭",
		label:      "ask",
		save:       askFlags.save,
		quiet:      askFlags.quiet || cfg.AskOutput() == config.AskOutputQuiet,
	})
}

// askOpts holds parameters for running the ask agent via the daemon.
type askOpts struct {
	question   string
	mode       ask.Mode
	project    string // alias (project mode)
	repo       string // ref (repo mode)
	rawURL     string // URL (url mode)
	workingDir string // CWD (general mode)
	maxSteps   int
	maxTokens  int
	emoji      string // optional display emoji shown before output
	label      string // display name shown in header (defaults to "ask")
	save       bool   // if true, pipe final answer to flicknote add
	quiet      bool   // if true, suppress streaming and print result after completion
}

// runAskAgent sends an ask request to the daemon and streams results to the terminal.
func runAskAgent(opts askOpts) error {
	label := opts.label
	if label == "" {
		label = "ask"
	}
	printAgentHeader(opts.emoji, label)

	// Build daemon request
	req := ask.Request{
		Question:   opts.question,
		Mode:       opts.mode,
		Project:    opts.project,
		Repo:       opts.repo,
		URL:        opts.rawURL,
		MaxSteps:   opts.maxSteps,
		MaxTokens:  opts.maxTokens,
		Save:       opts.save,
		Quiet:      opts.quiet,
		WorkingDir: opts.workingDir,
	}

	var finalResponse string
	var agentErr string

	eventHandler, sp := buildAskEventCallbacks(opts.quiet)
	defer sp.Stop()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	err := daemon.AskAgent(ctx, req, func(event ask.Event) {
		eventHandler(event)
		switch event.Type {
		case ask.EventDone:
			finalResponse = event.Response
		case ask.EventError:
			agentErr = event.Message
		}
	})
	if err != nil {
		return err // transport/daemon error
	}

	if opts.quiet && finalResponse != "" {
		fmt.Print(finalResponse)
		// Ensure trailing newline in quiet mode (verbose mode already streams deltas with newlines).
		if !strings.HasSuffix(finalResponse, "\n") {
			fmt.Println()
		}
	}

	if agentErr != "" {
		return fmt.Errorf("agent: %s", agentErr)
	}

	// Save to flicknote if requested.
	// done.Response is the full accumulated assistant text.
	if opts.save && finalResponse != "" {
		if saveErr := saveAskResponse(finalResponse); saveErr != nil {
			fmt.Fprintf(os.Stderr, "warning: failed to save to flicknote: %v\n", saveErr)
		}
	}

	return nil
}

// saveAskResponse pipes the agent's final answer to flicknote add.
func saveAskResponse(response string) error {
	if response == "" {
		return nil
	}
	cmd := exec.Command("flicknote", "add")
	cmd.Stdin = strings.NewReader(response)
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

// buildAskEventCallbacks returns an event handler and optional spinner for the given mode.
// In quiet mode all streaming events are suppressed and a spinner is started on TTY stderr.
// In verbose mode the full streaming callbacks are wired and spinner is nil.
func buildAskEventCallbacks(quiet bool) (func(ask.Event), *spinner) {
	if quiet {
		var sp *spinner
		if isTerminal(os.Stderr) {
			sp = startSpinner()
		}
		return func(ask.Event) {}, sp
	}
	return func(e ask.Event) {
		switch e.Type {
		case ask.EventDelta:
			renderDelta(e.Text)
		case ask.EventCommandStart:
			renderCommandStart(e.Command)
		case ask.EventCommandResult:
			renderCommandResult(e.Output, e.ExitCode)
		case ask.EventRetry:
			renderRetry(e.Reason, e.Step)
		case ask.EventStatus:
			fmt.Fprintf(os.Stderr, "%s\n", e.Message)
		}
	}, nil
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
