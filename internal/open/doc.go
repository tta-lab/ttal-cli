// Package open provides commands to open a task's working directory in an editor or shell.
//
// Given a task UUID it resolves the active worktree (or project root as fallback),
// then exec-replaces the current process with the configured editor or shell.
// Editor resolution honours TT_EDITOR, then EDITOR, then vi; shell resolution honours SHELL.
//
// Plane: shared
package open
