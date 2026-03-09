// Package sync deploys skills and subagent definitions to runtime directories.
//
// It reads canonical agent Markdown files and skill directories from configured
// source paths, generates runtime-specific variants (Claude Code, OpenCode,
// Codex), and writes or symlinks them to the appropriate destination directories.
// A clean operation removes stale managed files that no longer exist in source.
//
// Plane: shared
package sync
