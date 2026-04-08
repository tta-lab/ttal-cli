package cmdexec

import "strings"

// ClassifyShellCmd returns a refined tool identifier for a shell command string.
// For example, "ttal send --to foo" → "ttal:send", "flicknote add ..." → "flicknote:write".
// Returns "Bash" for unknown commands. Mirrors the refineBashTool switch in the watcher.
func ClassifyShellCmd(cmd string) string {
	if cmd == "" {
		return "Bash"
	}

	// Normalize: trim leading whitespace
	cmd = strings.TrimSpace(cmd)

	// Handle pipes: "echo x | flicknote add ..." or "cat <<'EOF' | flicknote add ..."
	// Use LastIndex to find the rightmost pipe — the final command in the pipeline.
	if idx := strings.LastIndex(cmd, "| "); idx >= 0 {
		cmd = strings.TrimSpace(cmd[idx+2:])
	}

	switch {
	case strings.HasPrefix(cmd, "ttal send "):
		return "ttal:send"
	case strings.HasPrefix(cmd, "ttal go "):
		return "ttal:route"
	case strings.HasPrefix(cmd, "flicknote add "),
		cmd == "flicknote add",
		strings.HasPrefix(cmd, "flicknote modify "),
		strings.HasPrefix(cmd, "flicknote append "),
		strings.HasPrefix(cmd, "flicknote insert "),
		strings.HasPrefix(cmd, "flicknote delete "),
		strings.HasPrefix(cmd, "flicknote rename "):
		return "flicknote:write"
	case strings.HasPrefix(cmd, "flicknote detail "),
		strings.HasPrefix(cmd, "flicknote content "),
		cmd == "flicknote list",
		strings.HasPrefix(cmd, "flicknote list "),
		strings.HasPrefix(cmd, "flicknote find "),
		strings.HasPrefix(cmd, "flicknote count"):
		return "flicknote:read"
	default:
		return "Bash"
	}
}
