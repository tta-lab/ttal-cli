// Package tmux provides helpers for managing tmux sessions, windows, and input delivery.
//
// It wraps the tmux CLI to create and kill sessions and windows, send literal text
// or raw control keys to panes, set session environment variables, and introspect
// the current session and window. Used by the daemon to route messages to agent
// sessions and by the worker spawner to launch new worker sessions.
//
// Plane: shared
package tmux
