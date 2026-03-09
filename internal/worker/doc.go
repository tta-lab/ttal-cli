// Package worker manages the full lifecycle of spawned worker sessions.
//
// It handles worker spawning (git worktree setup, tmux session creation, runtime
// launch command assembly via the gatekeeper deadman-switch wrapper), taskwarrior
// hook processing (on-add enrichment and on-modify cleanup triggers), worker listing,
// and session teardown. Workers run in isolated git worktrees within tmux sessions
// and are tracked via taskwarrior UDAs. The gatekeeper subprocess ensures the AI
// runtime binary is wrapped for safe lifecycle management.
//
// Plane: manager
package worker
