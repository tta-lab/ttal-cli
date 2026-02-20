package env

import (
	"os"
	"strings"
)

// ForSpawnCC returns the current environment with CC-specific vars removed,
// so spawned Claude Code subprocesses don't detect themselves as nested sessions.
func ForSpawnCC() []string {
	env := make([]string, 0, len(os.Environ()))
	for _, e := range os.Environ() {
		if strings.HasPrefix(e, "CLAUDECODE=") || strings.HasPrefix(e, "CLAUDE_CODE_ENTRYPOINT=") {
			continue
		}
		env = append(env, e)
	}
	return env
}
