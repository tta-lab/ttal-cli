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
		return fmt.Errorf("TTAL_AGENT_NAME not set and not in a tmux session — cannot route question to Telegram") //nolint:lll
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
		return fmt.Errorf("ask human: %w", err)
	}

	if result.Skipped {
		return fmt.Errorf("question skipped or timed out (no response within 5m)")
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
