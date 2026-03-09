// Package review manages reviewer session lifecycle for PR review.
//
// It spawns a new tmux window running a coding agent configured as a PR
// reviewer, builds the review prompt from config templates, and handles
// re-review requests by sending updated instructions to the existing window.
// Reviewers run in the worker plane using the team's worker_model.
//
// Plane: worker
package review
