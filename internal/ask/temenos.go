package ask

import (
	"context"
	"fmt"

	"github.com/tta-lab/logos"
)

// NewTemenosClient creates a CommandRunner and verifies the temenos daemon is reachable.
// The "daemon not running" hint is only shown when the health check actually fails.
func NewTemenosClient(ctx context.Context) (logos.CommandRunner, error) {
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
