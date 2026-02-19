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
	sendFrom  string
	sendTo    string
	sendStdin bool
)

var sendCmd = &cobra.Command{
	Use:   "send [message]",
	Short: "Send a message between agents or to a human",
	Long: `Send a message with explicit direction:

  --to <agent>                system/hook delivers to agent via Zellij
  --from <a> --to <b>         agent-to-agent via Zellij with attribution

Examples:
  ttal send --to kestrel "task started: implement auth"
  ttal send --from yuki --to kestrel "can you review my auth module?"
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

		return daemon.Send(daemon.SendRequest{
			From:    sendFrom,
			To:      sendTo,
			Message: message,
		})
	},
}

func init() {
	rootCmd.AddCommand(sendCmd)
	sendCmd.Flags().StringVar(&sendFrom, "from", "", "Source agent (for attribution)")
	sendCmd.Flags().StringVar(&sendTo, "to", "", "Receiving agent (routes via Zellij)")
	sendCmd.Flags().BoolVar(&sendStdin, "stdin", false, "Read message from stdin")
}
