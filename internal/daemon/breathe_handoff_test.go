package daemon

// buildBreatheHandoff was removed: context injection now happens in the CC SessionStart
// hook (ttal context command) rather than in the daemon. Route file consumption moved
// to cmd/context.go via route.Consume. Tests for context evaluation are in
// internal/sessionctx/sessionctx_test.go.
