// Package open provides commands to open a task's context in various ways.
//
// Four operations are supported: Editor (exec into $TT_EDITOR/$EDITOR/vi at the
// worktree root), Term (exec into $SHELL), Session (tmux attach to the worker's
// tmux session), and PR (open the task's pull request in the system browser).
// All operations resolve the task by UUID via taskwarrior.
//
// Plane: shared
package open
