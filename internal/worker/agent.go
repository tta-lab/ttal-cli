package worker

// CoderAgentName is the canonical agent name for worker sessions.
// Workers now launch code-lead (the orchestrator that delegates to
// coder/test-writer/doc-writer via ttal subagent run).
// Referenced by spawn.go (--agent flag, tmux window, TTAL_AGENT_NAME) and
// daemon/routing.go (workerWindow). Change here to rename everywhere.
const CoderAgentName = "code-lead"
