package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
	"github.com/tta-lab/ttal-cli/internal/daemon"
)

var (
	logTail  int
	logSince string
)

var logCmd = &cobra.Command{
	Use:   "log <project-alias>",
	Short: "Fetch pod logs via kubectl proxy",
	Long: `Fetch pod logs from Kubernetes via the daemon's kubectl proxy.

Requires the project to have k8s_app and k8s_namespace configured in projects.toml.

Examples:
  ttal log fb.ap              # last 100 lines
  ttal log fb.ap --tail 500  # custom line count
  ttal log fb.ap --since 5m  # logs from the last 5 minutes`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		alias := args[0]

		resp, err := daemon.KubeLog(daemon.KubeLogRequest{
			Alias: alias,
			Tail:  logTail,
			Since: logSince,
		})
		if err != nil {
			return fmt.Errorf("daemon request failed: %w", err)
		}
		if !resp.OK {
			return fmt.Errorf("%s", resp.Error)
		}
		fmt.Print(resp.Logs)
		return nil
	},
}

func init() {
	rootCmd.AddCommand(logCmd)
	logCmd.Flags().IntVar(&logTail, "tail", 100, "Number of log lines to fetch (default: 100)")
	logCmd.Flags().StringVar(&logSince, "since", "", "Show logs from the specified duration (e.g. 5m, 1h)")
}
