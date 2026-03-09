// Package team provides team-scoped agent session management.
//
// It resolves an agent name to a tmux session using the active team configuration,
// supporting both short "agent" and explicit "team:agent" addressing formats.
// The Attach function replaces the current process with a tmux attach-session call.
//
// Plane: shared
package team
