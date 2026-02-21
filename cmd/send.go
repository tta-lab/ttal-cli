package cmd

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"strings"

	"codeberg.org/clawteam/ttal-cli/internal/daemon"
	"github.com/spf13/cobra"
)

var (
	sendTo    string
	sendStdin bool
)

var sendCmd = &cobra.Command{
	Use:   "send [message]",
	Short: "Send a message between agents or to a human",
	Long: `Send a message with explicit direction:

  --to <agent>         delivers to agent via tmux
  --to human           sends to human via Telegram

Agent identity comes from TTAL_AGENT_NAME env var (set automatically in team tmux sessions).

Examples:
  ttal send --to kestrel "task started: implement auth"
  ttal send --to human "compact complete"
  echo "done" | ttal send --to kestrel --stdin`,
	// Skip database initialization
	PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
		return nil
	},
	PersistentPostRunE: func(cmd *cobra.Command, args []string) error {
		return nil
	},
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

		return daemon.Send(daemon.SendRequest{
			From:    from,
			To:      sendTo,
			Message: message,
		})
	},
}

func init() {
	rootCmd.AddCommand(sendCmd)
	sendCmd.Flags().StringVar(&sendTo, "to", "", "Receiving agent (routes via tmux)")
	sendCmd.Flags().BoolVar(&sendStdin, "stdin", false, "Read message from stdin")
}
