// Package launchcmd builds the shell command used to launch a worker or reviewer session.
//
// BuildCCDirectCommand constructs a gatekeeper-wrapped `claude --agent` invocation
// for Claude Code workers and reviewers. The trigger arg is sent as the first user
// message; agents are expected to receive `Run ttal context for your briefing` and
// invoke the orchestrator themselves.
// BuildLenosCommand mirrors BuildCCDirectCommand for the Lenos runtime.
// BuildCodexGatekeeperCommand constructs the gatekeeper-wrapped codex invocation
// using a task file as the initial prompt (Codex is dormant; legacy path).
//
// Plane: shared
package launchcmd
