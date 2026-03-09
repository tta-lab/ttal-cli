// Package runtime defines the coding agent runtime abstraction layer.
//
// It declares the Runtime type (claude-code, opencode, codex, openclaw) with
// parsing and validation helpers, and the Adapter interface that each runtime
// backend implements to provide a uniform API for starting, stopping, sending
// messages, receiving structured events, creating/resuming sessions, and health
// checks — enabling the daemon to resume interrupted conversations seamlessly.
//
// Plane: shared
package runtime
