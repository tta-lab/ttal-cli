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

var daemonSendFn = daemon.Send // inject point for tests

var sendCmd = &cobra.Command{
	Use:   "send [message]",
	Short: "Send a message between agents or to a human",
	Long: `Send a message with explicit direction:

  --to <agent>            delivers to agent via tmux
  --to <job_id>:<agent>   delivers to worker session
  --to <alias>            resolves human-first via humans.toml, then AI via team_path (see: ttal agent list)

Sender identity: TTAL_AGENT_NAME env var when set (agent or worker session); falls back to "system" for bare-shell sends, scripts, hooks, and automation.

Examples:
  ttal send --to kestrel "task started: implement auth"
  ttal send --to neil "compact complete"
  ttal send --to abc12345:coder "worker session message"

  # Piped stdin (single line):
  echo "done" | ttal send --to kestrel

  # Multiline via heredoc:
  cat <<'EOF' | ttal send --to neil
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
		if from == "" {
			from = "system"
		}

		usage.Log("send", sendTo)

		return daemonSendFn(daemon.SendRequest{
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
				"  Multiline: cat <<'END' | ttal send --to <name>\n"+
				"    ## Status\n    ...\n    END",
			sendExample)
	}
	return message, nil
}
