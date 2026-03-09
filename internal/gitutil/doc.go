// Package gitutil manages git worktree lifecycle for worker sessions.
//
// It provides DumpWorkerState (captures recent commits and status to a debug
// file), IsWorktreeClean (detects uncommitted changes), and RemoveWorktree
// (deletes a worktree directory, prunes metadata, and removes the worker
// branch). Used by the worker close and spawn paths.
//
// Plane: worker
package gitutil
