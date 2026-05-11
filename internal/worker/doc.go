// Package worker manages the full lifecycle of spawned workers.
//
// It handles worker spawning (git worktree setup, tmux window creation in the
// owner manager session, runtime launch command assembly via the gatekeeper
// deadman-switch wrapper), taskwarrior hook processing (on-add enrichment and
// on-modify cleanup triggers), worker listing, and teardown. Workers run in
// isolated git worktrees within named tmux windows in the owner's manager session
// (ttal-default-<owner>:<agent-name>) and are tracked via taskwarrior UDAs.
// The gatekeeper subprocess ensures the AI runtime binary is wrapped for safe
// lifecycle management.
//
// Plane: manager
package worker
