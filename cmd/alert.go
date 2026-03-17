package cmd

import (
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"
	"github.com/tta-lab/ttal-cli/internal/daemon"
	"github.com/tta-lab/ttal-cli/internal/flicktask"
	"github.com/tta-lab/ttal-cli/internal/notify"
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

		if alertToHuman {
			return notify.Send(message)
		}

		routed, err := alertToSpawner(cmd, message)
		if err != nil {
			return err // delivery was attempted but failed — don't silently fall back
		}
		if routed {
			return nil
		}

		// No spawner configured — fall through to Telegram
		return notify.Send(message)
	},
}

// alertToSpawner attempts to route the alert to the spawner agent.
//
//   - routed=false, err=nil  → no spawner configured, caller should fall back to Telegram
//   - routed=true,  err=nil  → delivered to spawner
//   - routed=false, err!=nil → spawner resolved but delivery failed — surface to caller
func alertToSpawner(cmd *cobra.Command, message string) (routed bool, err error) {
	sessionID := os.Getenv("TTAL_JOB_ID")
	if sessionID == "" {
		return false, nil
	}

	task, twErr := flicktask.ExportTaskBySessionID(sessionID, "pending")
	if twErr != nil {
		fmt.Fprintf(cmd.ErrOrStderr(), "warning: could not resolve task for alert routing: %v\n", twErr)
		return false, nil // can't determine spawner — fall back gracefully
	}

	if task.Spawner == "" {
		return false, nil
	}

	team, agent := parseSpawner(task.Spawner)

	// Append reply instructions
	message += fmt.Sprintf("\n\nReply to this worker: ttal send --to %s \"your message\"", sessionID)

	if sendErr := daemon.Send(daemon.SendRequest{
		From:    os.Getenv("TTAL_AGENT_NAME"),
		To:      agent,
		Team:    team,
		Message: message,
	}); sendErr != nil {
		return false, sendErr
	}

	return true, nil
}

// parseSpawner splits a "team:agent" string.
// Returns empty team when no colon is present — daemon.Send auto-fills Team from TTAL_TEAM.
func parseSpawner(s string) (team, agent string) {
	parts := strings.SplitN(s, ":", 2)
	if len(parts) == 2 {
		return parts[0], parts[1]
	}
	return "", s
}

func init() {
	rootCmd.AddCommand(alertCmd)
	alertCmd.Flags().BoolVar(&alertToHuman, "to-human", false, "Force delivery to Telegram instead of spawner agent")
}
