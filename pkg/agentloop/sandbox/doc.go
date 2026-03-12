// Package sandbox provides platform-aware sandboxed command execution.
//
// On Linux, it uses bubblewrap (bwrap) for namespace isolation.
// On macOS, it uses seatbelt (sandbox-exec) for kernel-level sandboxing.
// Both provide: deny-default filesystem access, explicit mount/path allowlists,
// network access, per-execution env vars, and timeout enforcement.
//
// Use New(Options) to get the appropriate sandbox for the current platform.
// ExecConfig carries per-execution env vars and mounts, and is threaded through
// context via ContextWithExecConfig / ExecConfigFromContext so tools can access
// it without explicit parameter threading.
//
// Plane: shared
package sandbox
