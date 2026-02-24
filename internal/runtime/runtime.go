package runtime

import "fmt"

// Runtime identifies which coding agent backend to use.
type Runtime string

const (
	ClaudeCode Runtime = "claude-code"
	OpenCode   Runtime = "opencode"
)

// All returns all valid runtime values.
func All() []Runtime {
	return []Runtime{ClaudeCode, OpenCode}
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
