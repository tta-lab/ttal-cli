// Package sandbox provides bubblewrap-based sandboxed command execution.
//
// It wraps bwrap to run bash commands in an isolated filesystem with network
// access but no host filesystem access beyond explicit mounts. ExecConfig
// carries per-execution env vars and mounts, and is threaded through context
// via ContextWithExecConfig / ExecConfigFromContext so tools can access it
// without explicit parameter threading.
//
// Plane: shared
package sandbox
