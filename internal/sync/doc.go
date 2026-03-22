// Package sync deploys skills, subagent definitions, and config TOMLs to runtime directories.
//
// It reads canonical agent Markdown files and skill directories from configured
// source paths, generates runtime-specific variants (Claude Code, Codex),
// and writes or symlinks them to the appropriate destination directories.
package sync
