// Package launchcmd builds the shell command used to launch a worker or reviewer session.
//
// BuildCCDirectCommand constructs a gatekeeper-wrapped `claude --agent` invocation
// for Claude Code workers and reviewers.
// BuildLenosCommand mirrors BuildCCDirectCommand for the Lenos runtime.
// BuildAgentLaunchCommand is the SSOT switch — chooses the command by runtime
// (Lenos or ClaudeCode); returns an error for unsupported runtimes (Codex).
// BuildEnvParts returns the SSOT env vars (TTAL_AGENT_NAME, TTAL_JOB_ID, TTAL_RUNTIME).
//
// Every spawned worker-plane agent receives ContextTrigger (`ttal context`) as
// its wake-orientation — no pre-rendered prompt files.
//
// Plane: shared
package launchcmd
