// Package launchcmd builds the shell command used to launch a worker session.
//
// BuildGatekeeperCommand constructs the full invocation string for a given
// runtime (ClaudeCode, OpenCode, Codex), embedding the ttal gatekeeper
// wrapper so the worker's lifecycle is managed before handing off to the
// AI runtime binary. Used by both the manager-plane worker spawner and the worker-plane reviewer.
//
// Plane: shared
package launchcmd
