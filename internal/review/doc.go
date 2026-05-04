// Package review manages reviewer session lifecycle for PR review.
//
// It spawns a new tmux window running a coding agent configured as a PR
// reviewer, and handles re-review requests by sending updated instructions
// to the existing window. Reviewers boot via ContextTrigger + ttal context
// (no pre-rendered prompt). Runtime is resolved from agentfs frontmatter
// (caller resolves via agentfs.ResolveRuntime, passed as rt param).
//
// Plane: worker
package review
