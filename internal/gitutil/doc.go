// Package gitutil manages git worktree lifecycle and credential injection for git operations.
//
// It provides DumpWorkerState (captures recent commits and status to a debug
// file), IsWorktreeClean (detects uncommitted changes), RemoveWorktree
// (deletes a worktree directory, prunes metadata, and removes the worker
// branch), and GitCredEnv (credential environment injection for git network
// operations). Used by the worker close, spawn, daemon, and ask paths.
//
// Plane: shared
package gitutil
