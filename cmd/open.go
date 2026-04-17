package cmd

import (
	"github.com/spf13/cobra"
	"github.com/tta-lab/ttal-cli/internal/open"
)

var openCmd = &cobra.Command{
	Use:   "open",
	Short: "Open task resources (PR, session, editor, term)",
	Long: `Open resources associated with a taskwarrior task.

These commands are designed to be called from taskwarrior-tui shortcuts:

  ttal open pr <uuid>        Open the Forgejo PR in browser
  ttal open session <uuid>   Attach to the tmux worker session
  ttal open editor <uuid>    Open the project/worktree in your editor
  ttal open term <uuid>      Open a shell in the worker directory`,
}

var openPRCmd = &cobra.Command{
	Use:   "pr <uuid>",
	Short: "Open PR in browser",
	Long: `Open the Forgejo PR associated with a task in the default browser.

Reads pr_id UDA and resolves project path from projects.toml, detects the git remote
to construct the PR URL, and opens it.

Environment variables:
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
	Short: "Attach to worker tmux session",
	Long: `Attach to the tmux session associated with a task's worker.

Derives session name from task UUID and exec's into tmux attach.

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

Detects worktree from branch UDA. Falls back to the project root
if no worktree is found.

Editor priority: TT_EDITOR > EDITOR > vi

Example:
  ttal open editor 12345678-1234-1234-1234-123456789abc`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		return open.Editor(args[0])
	},
}

var openTermCmd = &cobra.Command{
	Use:   "term <uuid>",
	Short: "Open terminal in worker directory",
	Long: `Open a shell in the task's working directory (worktree or project root).

Detects worktree from branch UDA. Falls back to the project root
if no worktree is found.

Shell priority: SHELL > /bin/sh

Example:
  ttal open term 12345678-1234-1234-1234-123456789abc`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		return open.Term(args[0])
	},
}

func init() {
	rootCmd.AddCommand(openCmd)

	openCmd.AddCommand(openPRCmd)
	openCmd.AddCommand(openSessionCmd)
	openCmd.AddCommand(openEditorCmd)
	openCmd.AddCommand(openTermCmd)
}
