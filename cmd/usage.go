package cmd

import (
	"bufio"
	"encoding/json"
	"io"
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
	info, err := os.Stdin.Stat()
	if err != nil || info.Mode()&os.ModeCharDevice != 0 {
		return "" // stat failed or no piped input
	}
	return extractIDFromReader(os.Stdin)
}

// extractIDFromReader reads the first line of r, parses as JSON, and returns
// the "id" field as a string. Returns "" on any failure. Extracted for testability.
func extractIDFromReader(r io.Reader) string {
	scanner := bufio.NewScanner(r)
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
