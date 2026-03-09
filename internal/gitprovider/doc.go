// Package gitprovider offers a provider-agnostic interface for git hosting operations.
//
// It defines the Provider interface covering PR lifecycle (create, edit, get,
// merge, comment) and CI status queries, with concrete implementations for
// Forgejo and GitHub. DetectProvider auto-selects the correct backend by
// inspecting the git remote URL of a working directory.
//
// Plane: shared
package gitprovider
