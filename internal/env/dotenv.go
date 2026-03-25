package env

import (
	"fmt"
	"log"

	"github.com/tta-lab/ttal-cli/internal/config"
)

// AllowedDotEnvParts loads ~/.config/ttal/.env and returns KEY=VALUE strings
// for vars that pass IsAllowedForSession. Logs a warning if .env fails to load.
// Used by both buildManagerAgentEnv and buildBreatheEnv to avoid duplicating the
// filter loop and the silent-error decision.
func AllowedDotEnvParts() []string {
	dotEnv, err := config.LoadDotEnv()
	if err != nil {
		log.Printf("[env] warning: failed to load .env, no secrets injected: %v", err)
		return nil
	}
	parts := make([]string, 0, len(dotEnv))
	for k, v := range dotEnv {
		if IsAllowedForSession(k) {
			parts = append(parts, fmt.Sprintf("%s=%s", k, v))
		}
	}
	return parts
}
