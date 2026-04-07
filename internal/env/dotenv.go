package env

import (
	"fmt"
	"log"
	"strings"

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

// AllowedDotEnvMap returns the same filtered .env vars as AllowedDotEnvParts,
// but as a map instead of a KEY=VALUE slice. Nil is returned if .env cannot be loaded.
func AllowedDotEnvMap() map[string]string {
	dotEnv, err := config.LoadDotEnv()
	if err != nil {
		log.Printf("[env] warning: failed to load .env, no secrets injected: %v", err)
		return nil
	}
	m := make(map[string]string)
	for k, v := range dotEnv {
		if IsAllowedForSession(k) {
			m[k] = v
		}
	}
	return m
}

// EnvSliceToMap converts a KEY=VALUE string slice to a map.
// Keys with no "=" separator are skipped. Empty values are preserved.
func EnvSliceToMap(parts []string) map[string]string {
	m := make(map[string]string, len(parts))
	for _, p := range parts {
		if k, v, ok := strings.Cut(p, "="); ok {
			m[k] = v
		}
	}
	return m
}
