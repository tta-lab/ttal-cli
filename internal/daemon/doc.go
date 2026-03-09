// Package daemon implements the long-running ttal manager-plane process.
//
// The daemon starts all agent sessions (tmux for Claude Code, HTTP adapters for
// OpenCode/OpenClaw), bridges messages between Telegram and agent runtimes via a
// Unix socket, watches JSONL output files to forward agent responses to Telegram,
// and manages the worker lifecycle through fsnotify-based cleanup and PR watchers.
// It is managed by launchd and handles all inter-agent and human-agent messaging
// for every configured team from a single process.
//
// Plane: manager
package daemon
