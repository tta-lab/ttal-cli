// Package sync deploys the global prompt, rules, and config TOMLs to runtime directories.
//
// It reads skill directories and config files from configured source paths,
// generates runtime-specific variants (Claude Code, Codex),
// and writes or symlinks them to the appropriate destination directories.
// Subagent and sandbox deployment are handled by einai (ei sync / ei sandbox sync).
//
// Plane: shared
package sync
