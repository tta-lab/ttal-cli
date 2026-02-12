package cmd

import (
	"github.com/guion-opensource/ttal-cli/internal/open"
	"github.com/spf13/cobra"
)

var openCmd = &cobra.Command{
	Use:   "open",
	Short: "Open task resources (PR, session, editor)",
	Long: `Open resources associated with a taskwarrior task.

These commands are designed to be called from taskwarrior-tui shortcuts:

  ttal open pr <uuid>        Open the Forgejo PR in browser
  ttal open session <uuid>   Attach to the zellij worker session
  ttal open editor <uuid>    Open the project/worktree in your editor`,
	// Skip database initialization — open commands use taskwarrior, not ttal's DB
	PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
		return nil
	},
	PersistentPostRunE: func(cmd *cobra.Command, args []string) error {
		return nil
	},
}

var openPRCmd = &cobra.Command{
	Use:   "pr <uuid>",
	Short: "Open PR in browser",
	Long: `Open the Forgejo PR associated with a task in the default browser.

Reads pr_id and project_path UDAs from the task, detects the git remote
to construct the PR URL, and opens it.

Environment variables:
  FORGEJO_URL            Forgejo instance URL (default: https://git.guion.io)
  FORGEJO_DEFAULT_OWNER  Fallback repo owner (default: neil)

Example:
  ttal open pr 12345678-1234-1234-1234-123456789abc`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		return open.PR(args[0])
	},
}

var openSessionCmd = &cobra.Command{
	Use:   "session <uuid>",
	Short: "Attach to worker zellij session",
	Long: `Attach to the zellij session associated with a task's worker.

Reads session_name UDA from the task and exec's into zellij attach.

Example:
  ttal open session 12345678-1234-1234-1234-123456789abc`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		return open.Session(args[0])
	},
}

var openEditorCmd = &cobra.Command{
	Use:   "editor <uuid>",
	Short: "Open project in editor",
	Long: `Open the task's project directory (or worktree) in your editor.

Detects worktree from session_name or branch UDAs. Falls back to the
project root if no worktree is found.

Editor priority: TT_EDITOR > EDITOR > vi

Example:
  ttal open editor 12345678-1234-1234-1234-123456789abc`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		return open.Editor(args[0])
	},
}

func init() {
	rootCmd.AddCommand(openCmd)

	openCmd.AddCommand(openPRCmd)
	openCmd.AddCommand(openSessionCmd)
	openCmd.AddCommand(openEditorCmd)
}
