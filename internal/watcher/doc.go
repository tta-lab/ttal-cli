// Package watcher tails Claude Code JSONL session files and forwards agent output to Telegram.
//
// It uses fsnotify to watch the ~/.claude/projects/ directories for registered agents,
// reads new bytes from tracked file offsets, and parses complete JSONL lines to extract
// assistant text blocks, tool invocations, and AskUserQuestion events. Detected events
// are dispatched via caller-provided callbacks to the daemon for Telegram delivery.
//
// Plane: manager
package watcher
