package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"github.com/tta-lab/ttal-cli/internal/daemon"
)

// runAskHuman implements `ttal ask --human "question" [--option ...]`.
// Posts to the daemon's /ask/human endpoint and prints the answer to stdout.
func runAskHuman(_ *cobra.Command, args []string, options []string) error {
	question := args[0]

	agentName := os.Getenv("TTAL_AGENT_NAME")
	tmuxEnv := os.Getenv("TMUX")
	if agentName == "" && tmuxEnv == "" {
		fmt.Fprintln(os.Stderr, "error: TTAL_AGENT_NAME not set and not in a tmux session — cannot route question to Telegram") //nolint:lll
		os.Exit(1)
	}

	tmuxSession := ""
	if agentName == "" {
		tmuxSession = resolveTmuxSession()
	}

	req := daemon.AskHumanRequest{
		Question:  question,
		Options:   options,
		AgentName: agentName,
		Session:   tmuxSession,
	}

	result, err := daemon.AskHuman(req)
	if err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}

	if result.Skipped {
		fmt.Fprintln(os.Stderr, "question skipped or timed out (no response within 5m)")
		os.Exit(1)
	}

	fmt.Print(result.Answer)
	return nil
}

// resolveTmuxSession returns an identifier for the current tmux session.
// Uses TMUX_PANE if set, otherwise falls back to the TMUX socket path value.
func resolveTmuxSession() string {
	if pane := os.Getenv("TMUX_PANE"); pane != "" {
		return pane
	}
	return os.Getenv("TMUX")
}
