package cmd

import (
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/spf13/cobra"
	"github.com/tta-lab/ttal-cli/internal/daemon"
)

var breatheCmd = &cobra.Command{
	Use:   "breathe [handoff-prompt]",
	Short: "Restart session with fresh context — exhale heavy context, take a fresh breath",
	Long: `Breathe triggers a context window refresh for the calling agent.

The agent writes a handoff prompt describing current state, decisions, and
next steps. ttal sends this to the daemon, which restarts the CC session
with the handoff as the initial message via --resume.

Usage from within an agent session:
  ttal breathe "# Handoff ..."
  echo "handoff content" | ttal breathe
  cat handoff.md | ttal breathe`,
	RunE: runBreathe,
}

func init() {
	rootCmd.AddCommand(breatheCmd)
}

func runBreathe(_ *cobra.Command, args []string) error {
	agent := os.Getenv("TTAL_AGENT_NAME")
	if agent == "" {
		return fmt.Errorf("TTAL_AGENT_NAME not set — ttal breathe must be called from within an agent session")
	}

	// Read handoff: positional args or stdin
	var handoff string
	if len(args) > 0 {
		handoff = strings.Join(args, " ")
	} else {
		data, err := io.ReadAll(os.Stdin)
		if err != nil {
			return fmt.Errorf("read stdin: %w", err)
		}
		handoff = strings.TrimSpace(string(data))
	}

	if handoff == "" {
		return fmt.Errorf("handoff prompt is required\n\n  Example: ttal breathe \"summary of current state and next steps\"")
	}

	if err := daemon.Breathe(daemon.BreatheRequest{
		Agent:       agent,
		Handoff:     handoff,
		SessionName: os.Getenv("TTAL_SESSION_NAME"),
	}); err != nil {
		return err
	}

	fmt.Fprintf(os.Stderr, "Handoff sent — taking a fresh breath...\n")
	return nil
}
