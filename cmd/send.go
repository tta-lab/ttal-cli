package cmd

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/spf13/cobra"
	"github.com/tta-lab/ttal-cli/internal/daemon"
)

var (
	sendTo    string
	sendStdin bool
)

var sendCmd = &cobra.Command{
	Use:   "send [message]",
	Short: "Send a message between agents or to a human",
	Long: `Send a message with explicit direction:

  --to <agent>         delivers to agent via tmux (uses TTAL_TEAM for team context)
  --to <team>:<agent>  delivers to agent in a specific team
  --to human           sends to human via Telegram

Agent identity comes from TTAL_AGENT_NAME env var (set automatically in team tmux sessions).
Team context comes from TTAL_TEAM env var, or can be specified with team:agent syntax.

Examples:
  ttal send --to kestrel "task started: implement auth"
  ttal send --to guion:astra "design review needed"
  ttal send --to human "compact complete"
  echo "done" | ttal send --to kestrel --stdin`,
	Args: cobra.ArbitraryArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		if sendTo == "" {
			return fmt.Errorf("--to is required (agent→Telegram is handled by the daemon JSONL watcher)")
		}

		var message string

		if sendStdin {
			data, err := io.ReadAll(bufio.NewReader(os.Stdin))
			if err != nil {
				return fmt.Errorf("failed to read stdin: %w", err)
			}
			message = strings.TrimRight(string(data), "\n")
		} else {
			if len(args) == 0 {
				return fmt.Errorf("message argument required (or use --stdin)")
			}
			message = strings.Join(args, " ")
		}

		if message == "" {
			return fmt.Errorf("message cannot be empty")
		}

		from := os.Getenv("TTAL_AGENT_NAME")
		if sendTo == "human" && from == "" {
			return fmt.Errorf("TTAL_AGENT_NAME env var required to send to human")
		}

		// Resolve team:agent syntax or fall back to TTAL_TEAM env var
		team := os.Getenv("TTAL_TEAM")
		to := sendTo
		if parts := strings.SplitN(sendTo, ":", 2); len(parts) == 2 {
			team = parts[0]
			to = parts[1]
		}

		return daemon.Send(daemon.SendRequest{
			From:    from,
			To:      to,
			Team:    team,
			Message: message,
		})
	},
}

func init() {
	rootCmd.AddCommand(sendCmd)
	sendCmd.Flags().StringVar(&sendTo, "to", "", "Receiving agent (routes via tmux)")
	sendCmd.Flags().BoolVar(&sendStdin, "stdin", false, "Read message from stdin")
}
