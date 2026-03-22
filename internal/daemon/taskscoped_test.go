package daemon

import "testing"

func TestBuildTaskScopedEnv(t *testing.T) {
	env := buildTaskScopedEnv("astra", "default", "1d87b1a8", "ts-1d87b1a8-astra", "")
	want := map[string]bool{
		"TTAL_AGENT_NAME=astra":               true,
		"TTAL_TEAM=default":                   true,
		"TTAL_JOB_ID=1d87b1a8":                true,
		"TTAL_SESSION_NAME=ts-1d87b1a8-astra": true,
		"TTAL_SESSION_MODE=task-scoped":       true,
	}
	for _, e := range env {
		delete(want, e)
	}
	for k := range want {
		t.Errorf("missing env var: %s", k)
	}
}
