package cmd

import (
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"
	"github.com/tta-lab/ttal-cli/internal/config"
	"github.com/tta-lab/ttal-cli/internal/daemon"
	"github.com/tta-lab/ttal-cli/internal/taskwarrior"
	"github.com/tta-lab/ttal-cli/internal/tmux"
)

var alertToHuman bool

var alertCmd = &cobra.Command{
	Use:   "alert [message]",
	Short: "Send a notification to the owner agent or team's notification bot",
	Long: `Send a short alert message. Routes to the owner agent if running
inside a worker session (has TTAL_JOB_ID and task has an owner set).
Falls back to the team's Telegram notification bot otherwise.

Use --to-human to force Telegram delivery even when an owner exists.

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
			return daemon.Notify(daemon.NotifyRequest{Message: message})
		}

		routed, err := alertToOwner(cmd, message)
		if err != nil {
			return err // delivery was attempted but failed — don't silently fall back
		}
		if routed {
			return nil
		}

		// No owner configured — fall through to daemon notification
		return daemon.Notify(daemon.NotifyRequest{Message: message})
	},
}

// alertToOwner attempts to route the alert to the owner agent.
//
//   - routed=false, err=nil  → no owner configured, caller should fall back to Telegram
//   - routed=true,  err=nil  → delivered to owner
//   - routed=false, err!=nil → owner resolved but delivery failed — surface to caller
func alertToOwner(cmd *cobra.Command, message string) (routed bool, err error) {
	sessionID := os.Getenv("TTAL_JOB_ID")
	if sessionID == "" {
		return false, nil
	}

	task, twErr := taskwarrior.ExportTaskByHexID(sessionID, "pending")
	if twErr != nil {
		fmt.Fprintf(cmd.ErrOrStderr(), "warning: could not resolve task for alert routing: %v\n", twErr)
		return false, nil // can't determine owner — fall back gracefully
	}

	if task.Owner == "" {
		return false, nil
	}

	// Append reply instructions
	addr := sessionID
	if agentName := os.Getenv("TTAL_AGENT_NAME"); agentName != "" {
		addr = sessionID + ":" + agentName
	}
	message += "\n\n" + daemon.ReplyHint(addr)

	cfg, cfgErr := config.Load()
	if cfgErr != nil {
		fmt.Fprintf(cmd.ErrOrStderr(), "warning: could not load config for alert routing: %v\n", cfgErr)
		return false, nil
	}
	team := cfg.TeamName()

	if sendErr := daemon.Send(daemon.SendRequest{
		From:    addr,
		To:      task.Owner,
		Team:    team,
		Message: message,
	}); sendErr != nil {
		return false, sendErr
	}

	return true, nil
}

func init() {
	rootCmd.AddCommand(alertCmd)
	alertCmd.Flags().BoolVar(&alertToHuman, "to-human", false, "Force delivery to Telegram instead of owner agent")
}
