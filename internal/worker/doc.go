// Package worker manages the full lifecycle of spawned worker sessions.
//
// It handles worker spawning (git worktree setup, tmux session creation, runtime
// launch command assembly), taskwarrior hook processing (on-add enrichment and
// on-modify cleanup triggers), worker listing, and session teardown. Workers run
// in isolated git worktrees within tmux sessions and are tracked via taskwarrior UDAs.
//
// Plane: manager
package worker
