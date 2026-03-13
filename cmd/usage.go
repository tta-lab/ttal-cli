package cmd

import (
	"bufio"
	"encoding/json"
	"os"

	"github.com/spf13/cobra"
	"github.com/tta-lab/ttal-cli/internal/usage"
)

var usageCmd = &cobra.Command{
	Use:   "usage",
	Short: "Tool usage tracking",
}

var usageLogCmd = &cobra.Command{
	Use:   "log <command> <subcommand>",
	Short: "Log a tool usage event",
	Long: `Log a tool usage event to the worklog database.

Reads JSON from stdin (first line) and extracts the "id" field as the target.
If stdin is empty or not JSON, logs without a target.

Designed for use in flicknote hooks:

  #!/bin/sh
  CMD=$(echo "$2" | sed 's/^command://')
  ttal usage log flicknote "$CMD"
  exit 0

Requires TTAL_AGENT_NAME in the environment. Silently skips if unset.`,
	Args: cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		command := args[0]
		subcommand := args[1]

		target := usageLogTarget
		if target == "" {
			target = extractTargetFromStdin()
		}

		usage.LogWith(command, subcommand, target)
		return nil
	},
}

var usageLogTarget string

// extractTargetFromStdin reads the first line of stdin, parses as JSON,
// and returns the "id" field. Returns "" on any failure.
func extractTargetFromStdin() string {
	info, _ := os.Stdin.Stat()
	if info.Mode()&os.ModeCharDevice != 0 {
		return "" // no piped input
	}

	scanner := bufio.NewScanner(os.Stdin)
	if !scanner.Scan() {
		return ""
	}

	var obj map[string]any
	if err := json.Unmarshal(scanner.Bytes(), &obj); err != nil {
		return ""
	}

	if id, ok := obj["id"].(string); ok {
		return id
	}
	return ""
}

func init() {
	usageLogCmd.Flags().StringVar(&usageLogTarget, "target", "", "explicit target (overrides stdin extraction)")
	usageCmd.AddCommand(usageLogCmd)
	rootCmd.AddCommand(usageCmd)
}
