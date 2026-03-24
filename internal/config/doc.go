// Package config loads and resolves the ttal configuration from ~/.config/ttal/config.toml.
//
// The active team is selected via the default_team field (falling back to "default").
// Resolved fields (chat ID, paths, runtimes, models, prompt templates) are promoted
// to a single Config value so callers do not need to know which team is active.
// Also provides DaemonConfig for loading all teams simultaneously, and helpers for
// resolving data directories, projects paths, and prompt templates from prompts.toml
// and roles.toml.
//
// Plane: shared
package config
