// Package breathe handles session handoff for context window refresh.
//
// When a worker's context window is getting full, the worker calls
// `ttal breathe` with a handoff prompt. The daemon persists the handoff
// to diary and restarts the worker session. For manager agents, context
// injection is handled by the CC SessionStart hook (ttal context) which
// evaluates breathe_context commands at session startup.
//
// Plane: worker
package breathe
