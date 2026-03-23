package worker

// CoderAgentName is the canonical agent name for worker (coder) sessions.
// Referenced by spawn.go (--agent flag, tmux window, TTAL_AGENT_NAME) and
// daemon/routing.go (workerWindow). Change here to rename everywhere.
const CoderAgentName = "coder"
