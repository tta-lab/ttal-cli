// Package env provides environment variable helpers for spawning subprocesses.
//
// ForSpawnCC strips Claude Code session markers (CLAUDECODE, CLAUDE_CODE_ENTRYPOINT)
// from the current process environment so that spawned Claude Code workers do not
// detect themselves as nested sessions and alter their behaviour accordingly.
//
// Plane: shared
package env
