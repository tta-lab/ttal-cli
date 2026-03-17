// Package doctor runs diagnostic checks on the ttal installation and configuration.
//
// Checks prerequisites (tmux, flicktask, git, ffmpeg), config.toml validity,
// TaskChampion sync credentials, the project store, daemon health, environment
// variables, voice server status, and Claude Code integration. Each check returns
// an OK, warn, or error result. With --fix, doctor auto-remediates common issues
// such as creating missing config files and installing flicktask hooks.
//
// Plane: shared
package doctor
