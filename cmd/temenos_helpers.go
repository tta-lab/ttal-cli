package cmd

import (
	"context"
	"fmt"
	"strings"

	"github.com/tta-lab/logos"
)

// newTemenosClient creates a CommandRunner and verifies the daemon is reachable.
// The "daemon not running" hint is only shown when the health check actually fails.
func newTemenosClient(ctx context.Context) (logos.CommandRunner, error) {
	tc, err := logos.NewClient("")
	if err != nil {
		return nil, fmt.Errorf("connect to temenos daemon: %w", err)
	}
	if hc, ok := tc.(interface{ Health(context.Context) error }); ok {
		if err := hc.Health(ctx); err != nil {
			return nil, fmt.Errorf("temenos daemon unreachable: %w\n\n"+
				"Is the daemon running? Try: temenos daemon install && temenos daemon start", err)
		}
	}
	return tc, nil
}

// flushAgentResult prints a trailing newline if the response didn't end with one,
// then returns the error with an actionable hint when the step limit is reached.
func flushAgentResult(result *logos.RunResult, err error) error {
	if result != nil && result.Response != "" && !strings.HasSuffix(result.Response, "\n") {
		fmt.Println()
	}
	if err != nil {
		if strings.Contains(err.Error(), "max steps") {
			return fmt.Errorf("agent loop: %w\n\nTip: increase the limit with --max-steps", err)
		}
		return fmt.Errorf("agent loop: %w", err)
	}
	return nil
}
