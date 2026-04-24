package cmd

import (
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"
	"github.com/tta-lab/ttal-cli/internal/daemon"
	"github.com/tta-lab/ttal-cli/internal/usage"
)

const sendExample = `ttal send --to kestrel "message"
  ttal send --to abc12345:coder "message"    # worker session`

var sendTo string

var sendCmd = &cobra.Command{
	Use:   "send [message]",
	Short: "Send a message between agents or to a human",
	Long: `Send a message with explicit direction:

  --to <agent>            delivers to agent via tmux
  --to <job_id>:<agent>   delivers to worker session
  --to human              sends to human via Telegram

Agent identity comes from TTAL_AGENT_NAME env var (set automatically in team tmux sessions).

Examples:
  ttal send --to kestrel "task started: implement auth"
  ttal send --to human "compact complete"
  ttal send --to abc12345:coder "worker session message"

  # Piped stdin (single line):
  echo "done" | ttal send --to kestrel

  # Multiline via heredoc:
  cat <<'EOF' | ttal send --to human
  ## Status
  Review complete — 2 findings.
  EOF`,
	Args: cobra.ArbitraryArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		if sendTo == "" {
			return fmt.Errorf("--to is required\n\n  Example: %s", sendExample)
		}

		message, err := resolveSendMessage(args)
		if err != nil {
			return err
		}

		from := os.Getenv("TTAL_AGENT_NAME")
		jobID := os.Getenv("TTAL_JOB_ID")
		if jobID != "" && from != "" {
			from = jobID + ":" + from
		}
		if sendTo == "human" && from == "" {
			return fmt.Errorf("TTAL_AGENT_NAME not set — this command sends to Telegram and needs agent identity\nThis is set automatically in agent sessions") //nolint:lll
		}

		usage.Log("send", sendTo)

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
}

// resolveSendMessage picks the message body from positional args first, falling
// back to piped stdin only when no args are given. The args-first order is
// deliberate: callers launched under pueue/systemd/launchd inherit a stdin pipe
// FD that no one writes to, and io.ReadAll on that FD blocks forever. Reading
// stdin only when args are empty preserves the `echo ... | ttal send` ergonomic
// while letting positional-arg callers (e.g. ei .sh scripts) finish promptly.
func resolveSendMessage(args []string) (string, error) {
	var message string
	if len(args) > 0 {
		message = strings.Join(args, " ")
	} else {
		piped, err := readStdinIfPiped()
		if err != nil {
			return "", fmt.Errorf("read stdin: %w", err)
		}
		message = piped
	}

	if message == "" {
		return "", fmt.Errorf(
			"message required (positional argument or piped stdin)\n\n"+
				"  Example: %s\n"+
				"  Multiline: cat <<'END' | ttal send --to human\n"+
				"    ## Status\n    ...\n    END",
			sendExample)
	}
	return message, nil
}
