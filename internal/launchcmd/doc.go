// Package launchcmd builds the shell command used to launch a worker or reviewer session.
//
// BuildResumeCommand constructs a gatekeeper-wrapped claude --resume invocation
// for Claude Code workers and reviewers (JSONL session pattern).
// BuildCodexGatekeeperCommand constructs the legacy gatekeeper-wrapped codex
// invocation for Codex workers until #321 (Codex JSONL resume support) lands.
package launchcmd
