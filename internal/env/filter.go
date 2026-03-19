package env

import "strings"

// IsAllowedForSession returns true for env vars that workers/agents
// are permitted to receive. All other .env vars are blocked — their
// operations are proxied through the daemon for token isolation.
func IsAllowedForSession(key string) bool {
	// Credential-like suffixes are always blocked, even with TTAL_ prefix.
	// The TTAL_ namespace is for runtime metadata (team, job ID, agent name),
	// not credentials. A .env with TTAL_FORGEJO_TOKEN must not reach workers.
	if strings.HasSuffix(key, "_TOKEN") || strings.HasSuffix(key, "_SECRET") || strings.HasSuffix(key, "_PASSWORD") {
		return false
	}
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
