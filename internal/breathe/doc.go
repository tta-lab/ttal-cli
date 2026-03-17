// Package breathe handles session handoff for context window refresh.
//
// When an agent's context window is getting full, the agent calls
// `ttal breathe` with a handoff prompt. The daemon writes a synthetic
// CC session JSONL and restarts the agent with --resume, giving it
// a fresh context window with the handoff as the starting message.
//
// Plane: manager
package breathe
