package env

import "strings"

// IsAllowedForSession returns true for env vars that workers/agents
// are permitted to receive. All other .env vars are blocked — their
// operations are proxied through the daemon for token isolation.
func IsAllowedForSession(key string) bool {
	// ttal runtime vars (TTAL_TEAM, TTAL_JOB_ID, etc.)
	if strings.HasPrefix(key, "TTAL_") {
		return true
	}
	switch key {
	case "TASKRC", // taskwarrior config
		"FORGEJO_URL",     // PR URL construction (not a credential)
		"MINIMAX_API_KEY", // ttal ask — LLM provider
		"MINIMAX_API_URL", // ttal ask — LLM endpoint
		"BRAVE_API_KEY":   // ttal ask — web search
		return true
	}
	return false
}
