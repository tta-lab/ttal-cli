package cmd

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"
	"github.com/tta-lab/ttal-cli/internal/notify"
	"github.com/tta-lab/ttal-cli/internal/tmux"
)

var alertCmd = &cobra.Command{
	Use:   "alert [message]",
	Short: "Send a notification to the team's notification bot",
	Long: `Send a short alert message via the notification bot.
Automatically prepends the current tmux session name so the recipient
knows which worker is sending the alert.

Examples:
  ttal alert "build complete"
  ttal alert "blocked: need design input on auth flow"`,
	Args: cobra.MinimumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		message := strings.Join(args, " ")

		sessionName, err := tmux.CurrentSession()
		if err == nil && sessionName != "" {
			message = fmt.Sprintf("[%s] %s", sessionName, message)
		}

		return notify.Send(message)
	},
}

func init() {
	rootCmd.AddCommand(alertCmd)
}
