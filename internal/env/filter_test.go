package env

import "testing"

func TestIsAllowedForSession(t *testing.T) {
	allowed := []string{
		"TTAL_TEAM", "TTAL_JOB_ID", "TTAL_AGENT_NAME",
		"TASKRC",
		"FORGEJO_URL",
		"MINIMAX_API_KEY", "MINIMAX_API_URL",
		"BRAVE_API_KEY",
	}
	for _, k := range allowed {
		if !IsAllowedForSession(k) {
			t.Errorf("expected %s to be allowed", k)
		}
	}

	blocked := []string{
		"FORGEJO_TOKEN", "FORGEJO_ACCESS_TOKEN",
		"GITHUB_TOKEN",
		"WOODPECKER_TOKEN", "WOODPECKER_URL",
		"KESTREL_BOT_TOKEN", "ATHENA_BOT_TOKEN",
		"ANTHROPIC_API_KEY",
		"SOME_RANDOM_SECRET",
	}
	for _, k := range blocked {
		if IsAllowedForSession(k) {
			t.Errorf("expected %s to be blocked", k)
		}
	}
}
