package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"github.com/tta-lab/ttal-cli/internal/daemon"
)

var breatheFn = daemon.Breathe

var breatheCmd = &cobra.Command{
	Use:   "breathe",
	Short: "Restart session with fresh context — exhale heavy context, take a fresh breath",
	Long: `Restart your session with a fresh context window.

Handoff persistence is agent-side: write your handoff to your diary
BEFORE calling breathe, then call breathe with no arguments. The daemon
respawns your session and the new instance reads diary on wake via
ttal context.

Run "skill get breathe" for the full ritual.

Examples:
  ttal breathe                    # self-breathe (uses TTAL_AGENT_NAME)
  ttal breathe --agent kestrel    # force-breathe another agent`,
	RunE: runBreathe,
}

func init() {
	rootCmd.AddCommand(breatheCmd)
	breatheCmd.Flags().String("agent", "", "Force-breathe another agent (escape hatch for inter-agent use)")
}

func resolveBreatheTarget(cmd *cobra.Command, args []string) (string, error) {
	if len(args) > 0 {
		return "", fmt.Errorf("ttal breathe no longer accepts handoff arguments" +
			" — write your handoff to diary first, then run `ttal breathe`. See: skill get breathe")
	}
	if agent, _ := cmd.Flags().GetString("agent"); agent != "" {
		return agent, nil
	}
	if env := os.Getenv("TTAL_AGENT_NAME"); env != "" {
		return env, nil
	}
	return "", fmt.Errorf("must run from within an agent session, or pass --agent <name>")
}

func runBreathe(cmd *cobra.Command, args []string) error {
	target, err := resolveBreatheTarget(cmd, args)
	if err != nil {
		return err
	}

	if err := breatheFn(daemon.BreatheRequest{
		Agent:       target,
		SessionName: os.Getenv("TTAL_SESSION_NAME"),
	}); err != nil {
		return err
	}

	fmt.Fprintf(os.Stderr, "Breathing — see you on the other side...\n")
	return nil
}
