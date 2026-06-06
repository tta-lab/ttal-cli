// Package project resolves ttal project aliases and paths via the `project` CLI binary.
//
// The `project` binary (from organon) provides a read-only JSON API over
// ~/.config/ttal/projects.toml. This package wraps it for ttal's internal
// callers, adding ttal-specific heuristics (contains-fallback, hierarchical
// fallback, worktree alias extraction) on top.
//
// Plane: shared
package project
