// Package doctor runs diagnostic checks on the ttal installation and configuration.
//
// Checks prerequisites (tmux, task, git, ffmpeg), config.toml validity, taskwarrior
// UDA definitions, TaskChampion sync credentials, the project store, daemon health,
// environment variables, voice server status, and Claude Code integration. Each check
// returns an OK, warn, or error result. With --fix, doctor auto-remediates common
// issues such as creating missing config files and injecting missing include lines.
//
// Plane: shared
package doctor
