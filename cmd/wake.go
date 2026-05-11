package cmd

import (
	"os"

	"github.com/spf13/cobra"
	"github.com/tta-lab/ttal-cli/internal/owner"
	"github.com/tta-lab/ttal-cli/internal/runtime"
	"github.com/tta-lab/ttal-cli/internal/sendfmt"
)

// wakeTrigger is the fixed trigger message sent to every spawned or breathed agent.
const wakeTrigger = "Run `ttal context` for your briefing, then act on the role prompt."

// wakeCmd outputs the properly formatted trigger prompt for the current agent.
var wakeCmd = &cobra.Command{
	Use:   "wake",
	Short: "Output formatted trigger prompt for current agent",
	Long: `Output a formatted trigger prompt wrapped in the standard sendfmt envelope.

Manager plane: resolves owner from admin human alias.
Worker plane: resolves owner from the task owner UDA (TTAL_JOB_ID).

Output format:
  <- telegram:<owner> [HH:MM:SS] Run ` + "`ttal context`" + ` for your briefing, then act on the role prompt.

  <i>--- Reply with:
  cat <<'EOF' | ttal send --to <owner>
  your message
  EOF</i>`,
	RunE: func(cmd *cobra.Command, args []string) error {
		ownerName := owner.ResolveOwner()

		envelope := sendfmt.Envelope{
			Channel:      "telegram",
			SenderName:   ownerName,
			Body:         wakeTrigger,
			ReplyAlias:   ownerName,
			ReplyRuntime: wakeReplyRuntime(),
		}

		_, err := os.Stdout.WriteString(sendfmt.Format(envelope) + "\n")
		return err
	},
}

func wakeReplyRuntime() runtime.Runtime {
	rt, err := runtime.Parse(os.Getenv("TTAL_RUNTIME"))
	if err != nil {
		return runtime.ClaudeCode
	}
	return rt
}

func init() {
	rootCmd.AddCommand(wakeCmd)
}
