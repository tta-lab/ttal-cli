// Package config loads and resolves the ttal configuration from ~/.config/ttal/config.toml.
//
// Supports multi-team setups where the active team is selected via the TTAL_TEAM
// environment variable or the default_team field. Resolved fields (chat ID, paths,
// runtimes, models, prompt templates) are promoted to a single Config value so
// callers do not need to know which team is active. Also provides DaemonConfig
// for loading all teams simultaneously, and helpers for resolving data directories,
// projects paths, and prompt templates from prompts.toml and roles.toml.
//
// Plane: shared
package config
