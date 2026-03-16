package cmd

import (
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"
	"github.com/tta-lab/ttal-cli/internal/daemon"
	"github.com/tta-lab/ttal-cli/internal/notify"
	"github.com/tta-lab/ttal-cli/internal/taskwarrior"
	"github.com/tta-lab/ttal-cli/internal/tmux"
)

var alertToHuman bool

var alertCmd = &cobra.Command{
	Use:   "alert [message]",
	Short: "Send a notification to the spawner agent or team's notification bot",
	Long: `Send a short alert message. Routes to the spawner agent if running
inside a worker session (has TTAL_JOB_ID and task has a spawner set).
Falls back to the team's Telegram notification bot otherwise.

Use --to-human to force Telegram delivery even when a spawner exists.

Examples:
  ttal alert "build complete"
  ttal alert "blocked: need design input on auth flow"
  ttal alert --to-human "urgent: production issue"`,
	Args: cobra.MinimumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		message := strings.Join(args, " ")

		sessionName, err := tmux.CurrentSession()
		if err != nil {
			fmt.Fprintf(cmd.ErrOrStderr(), "warning: could not get tmux session name: %v\n", err)
		} else if sessionName != "" {
			message = fmt.Sprintf("[%s] %s", sessionName, message)
		}

		// Route to spawner agent unless --to-human is set
		if !alertToHuman {
			if err := alertToSpawner(cmd, message); err == nil {
				return nil // delivered to spawner
			}
			// Fall through to Telegram on any error
		}

		return notify.Send(message)
	},
}

// alertToSpawner attempts to route the alert to the spawner agent.
// Returns nil on success, error if spawner can't be resolved or delivery fails.
func alertToSpawner(cmd *cobra.Command, message string) error {
	sessionID := os.Getenv("TTAL_JOB_ID")
	if sessionID == "" {
		return fmt.Errorf("no TTAL_JOB_ID")
	}

	task, err := taskwarrior.ExportTaskBySessionID(sessionID, "pending")
	if err != nil {
		fmt.Fprintf(cmd.ErrOrStderr(), "warning: could not resolve task for alert routing: %v\n", err)
		return err
	}

	spawner := task.Spawner
	if spawner == "" {
		return fmt.Errorf("no spawner on task")
	}

	// Parse team:agent — default to TTAL_TEAM if no team prefix
	team, agent := parseSpawner(spawner)

	// Append reply instructions
	message += fmt.Sprintf("\n\nReply to this worker: ttal send --to %s \"your message\"", sessionID)

	return daemon.Send(daemon.SendRequest{
		To:      agent,
		Team:    team,
		Message: message,
	})
}

// parseSpawner splits a "team:agent" string. If no colon is present,
// falls back to TTAL_TEAM env var (or "default") as the team.
func parseSpawner(s string) (team, agent string) {
	parts := strings.SplitN(s, ":", 2)
	if len(parts) == 2 {
		return parts[0], parts[1]
	}
	team = os.Getenv("TTAL_TEAM")
	if team == "" {
		team = "default"
	}
	return team, s
}

func init() {
	rootCmd.AddCommand(alertCmd)
	alertCmd.Flags().BoolVar(&alertToHuman, "to-human", false, "Force delivery to Telegram instead of spawner agent")
}
