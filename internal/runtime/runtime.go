package runtime

import "fmt"

// Runtime identifies which coding agent backend to use.
type Runtime string

const (
	ClaudeCode Runtime = "claude-code"
	OpenCode   Runtime = "opencode"
	Codex      Runtime = "codex"
	OpenClaw   Runtime = "openclaw"
)

// All returns all valid runtime values.
func All() []Runtime {
	return []Runtime{ClaudeCode, OpenCode, Codex, OpenClaw}
}

// Values returns all valid runtime strings (for ent schema enum).
func Values() []string {
	all := All()
	out := make([]string, len(all))
	for i, r := range all {
		out[i] = string(r)
	}
	return out
}

// Parse converts a string to a Runtime, defaulting to ClaudeCode.
// Accepts aliases: "cc" for claude-code, "oc" for opencode.
func Parse(s string) (Runtime, error) {
	switch s {
	case "", string(ClaudeCode), "cc":
		return ClaudeCode, nil
	case string(OpenCode), "oc":
		return OpenCode, nil
	case string(Codex), "cx":
		return Codex, nil
	case string(OpenClaw), "oclw":
		return OpenClaw, nil
	default:
		return "", fmt.Errorf("unknown runtime: %q (valid: %s)", s, joinRuntimes())
	}
}

// Validate checks that a string is a valid runtime value.
func Validate(s string) error {
	for _, r := range All() {
		if s == string(r) {
			return nil
		}
	}
	return fmt.Errorf("unknown runtime %q (available: %s)", s, joinRuntimes())
}

// IsWorkerRuntime returns true if the runtime can be used for workers.
// OpenClaw is agent-only — workers always use CC/OC/Codex.
func (r Runtime) IsWorkerRuntime() bool {
	switch r {
	case ClaudeCode, OpenCode, Codex:
		return true
	default:
		return false
	}
}

// NeedsPort returns true if the runtime requires an explicit port for its HTTP server.
// OpenCode and Codex run HTTP serve processes; CC and OpenClaw do not.
func (r Runtime) NeedsPort() bool {
	switch r {
	case OpenCode, Codex:
		return true
	default:
		return false
	}
}

func joinRuntimes() string {
	all := All()
	s := ""
	for i, r := range all {
		if i > 0 {
			s += ", "
		}
		s += string(r)
	}
	return s
}
