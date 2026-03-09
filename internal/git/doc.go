// Package git provides low-level git repository helpers.
//
// It currently exposes FindRoot, which resolves the git repository root for
// any path inside a working tree by running "git rev-parse --show-toplevel".
// Used by worker spawn/close and CLI commands that need the repo root before
// operating on worktrees or remotes.
//
// Plane: shared
package git
