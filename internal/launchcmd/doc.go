// Package launchcmd builds the shell command used to launch a worker or reviewer session.
//
// BuildCCDirectCommand constructs a gatekeeper-wrapped claude --agent invocation
// for Claude Code workers and reviewers. Context is injected via the CC SessionStart
// hook (ttal context) rather than a synthetic JSONL session.
// BuildCodexGatekeeperCommand constructs the legacy gatekeeper-wrapped codex
// invocation for Codex workers until #321 (Codex context hook support) lands.
//
// Plane: shared
package launchcmd
