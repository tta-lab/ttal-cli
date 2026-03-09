// Package scaffold applies workspace templates from the ttal-templates repository.
//
// It clones or updates a local cache of the templates repo, lists available
// scaffolds by inspecting their README metadata, and copies a chosen scaffold
// (plus the shared docs/ directory) into a target workspace directory. Used by
// the CLI's init/scaffold commands to bootstrap new agent workspaces.
//
// Plane: shared
package scaffold
