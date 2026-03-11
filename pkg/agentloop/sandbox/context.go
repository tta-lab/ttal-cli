package sandbox

import "context"

type contextKey string

const execConfigKey contextKey = "execConfig"

// ContextWithExecConfig stores an ExecConfig in the context.
func ContextWithExecConfig(ctx context.Context, cfg *ExecConfig) context.Context {
	return context.WithValue(ctx, execConfigKey, cfg)
}

// ExecConfigFromContext retrieves the ExecConfig from the context.
// Returns nil if not set.
func ExecConfigFromContext(ctx context.Context) *ExecConfig {
	cfg, _ := ctx.Value(execConfigKey).(*ExecConfig)
	return cfg
}
