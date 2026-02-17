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

  --from <agent>              agent speaks to human via Telegram
  --to <agent>                system/hook delivers to agent via Zellij
  --from <a> --to <b>         agent-to-agent via Zellij with attribution

Examples:
  ttal send --from kestrel "PR #42 ready for review"
  ttal send --to kestrel "task started: implement auth"
  ttal send --from yuki --to kestrel "can you review my auth module?"
  echo "done" | ttal send --from kestrel --stdin`,
	// Skip database initialization
	PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
		return nil
	},
	PersistentPostRunE: func(cmd *cobra.Command, args []string) error {
		return nil
	},
	Args: cobra.ArbitraryArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		if sendFrom == "" && sendTo == "" {
			return fmt.Errorf("must specify --from, --to, or both")
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
	sendCmd.Flags().StringVar(&sendFrom, "from", "", "Sending agent (routes via Telegram)")
	sendCmd.Flags().StringVar(&sendTo, "to", "", "Receiving agent (routes via Zellij)")
	sendCmd.Flags().BoolVar(&sendStdin, "stdin", false, "Read message from stdin")
}
