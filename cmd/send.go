package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"github.com/tta-lab/ttal-cli/internal/daemon"
	"github.com/tta-lab/ttal-cli/internal/usage"
)

const sendExample = `cat <<'EOF' | ttal send --to kestrel
message
EOF
  cat <<'EOF' | ttal send --to abc12345:coder
  worker session message
  EOF`

var sendTo string

var daemonSendFn = daemon.Send // inject point for tests

var sendCmd = &cobra.Command{
	Use:   "send",
	Short: "Send a message between agents or to a human",
	Long: `Send a message with explicit direction:

  --to <agent>            delivers to agent via tmux
  --to <job_id>:<agent>   delivers to worker session
  --to <alias>            resolves human-first via humans.toml, then AI via team_path (see: ttal agent list)

Sender identity: TTAL_AGENT_NAME env var when set; falls back to "system" for bare
  shell sends, scripts, hooks, and automation.

Boundary:
  This CLI reads message bodies from stdin only. It is intended for controlled
  agent runtimes that invoke shell commands and provide heredoc/pipe input.
  Other process-style integrations should call the ttal daemon directly rather
  than shelling out to ttal send.

Examples:
  cat <<'EOF' | ttal send --to kestrel
  task started: implement auth
  EOF

  cat <<'EOF' | ttal send --to <human-alias>
  compact complete
  EOF

  cat <<'EOF' | ttal send --to abc12345:coder
  worker session message
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
			From:          from,
			To:            sendTo,
			Message:       message,
			UserInitiated: true,
		})
	},
}

func init() {
	rootCmd.AddCommand(sendCmd)
	sendCmd.Flags().StringVar(&sendTo, "to", "", "Receiving agent (routes via tmux)")
}

// resolveSendMessage enforces the stdin-only contract for ttal send.
//
// Boundary: this command is for controlled agent runtimes that can provide a
// heredoc or pipe. It is intentionally not a generic "ttal send \"message\""
// wrapper for arbitrary process callers; those should reach the daemon API
// directly instead of going through shell argument parsing here.
//
// Positional args therefore fail fast with actionable heredoc guidance.
func resolveSendMessage(args []string) (string, error) {
	if len(args) > 0 {
		return "", fmt.Errorf(
			"ttal send reads stdin only; positional arguments are not supported\n\n" +
				"  Use heredoc:\n" +
				"    cat <<'EOF' | ttal send --to <name>\n" +
				"    message\n" +
				"    EOF")
	}

	piped, err := readStdinIfPiped()
	if err != nil {
		return "", fmt.Errorf("read stdin: %w", err)
	}

	message := piped
	if message == "" {
		return "", fmt.Errorf(
			"message required on stdin\n\n"+
				"  Example: %s\n"+
				"  Multiline: cat <<'END' | ttal send --to <name>\n"+
				"    ## Status\n    ...\n    END",
			sendExample)
	}
	return message, nil
}
