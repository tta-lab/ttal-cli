// Package scaffold applies workspace templates to bootstrap new agent workspaces.
//
// Templates live in the templates/ directory of the ttal-cli repo. When running
// from a cloned repo, FindTemplatesDir resolves templates locally — no network
// needed. For brew-installed users, it falls back to a cached clone of the remote
// templates repo. Used by the CLI's init/onboard commands.
//
// Plane: shared
package scaffold
