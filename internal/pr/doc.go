// Package pr implements pull request operations for worker sessions.
//
// It resolves a PR context from the TTAL_JOB_ID environment variable, then exposes
// operations to create, modify, comment on, and squash-merge PRs via the gitprovider
// abstraction. Merging is gated on reviewer LGTM approval recorded in the task's pr_id UDA.
//
// Plane: worker
package pr
