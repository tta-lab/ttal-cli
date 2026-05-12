// Package sync deploys the global prompt, rules, skills, and agent identities to runtime directories.
//
// It reads skill directories and agent/rule files from configured source paths,
// generates runtime-specific variants (Claude Code, Codex),
// and writes or symlinks them to the appropriate destination directories.
// TTAL config TOMLs are managed outside ttal sync.
//
// Plane: shared
package sync
