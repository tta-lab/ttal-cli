package promptrender

import (
	"context"
	"log"
	"os"
	"os/exec"
	"strings"
	"time"
)

const commandTimeout = 30 * time.Second

// RenderTemplate walks a prompt template line-by-line.
// Lines starting with "$ " are executed as shell commands (via sh -c).
// All other lines are passed through as-is.
// Template vars ({{agent-name}}, {{team-name}}) are expanded before execution.
// env is a list of KEY=VALUE strings merged into the subprocess environment for $ commands.
// Hook-derived vars override any pre-existing session env vars of the same name.
// Non-zero exit commands are logged and skipped (output not included).
// Empty command output is omitted (no blank sections).
func RenderTemplate(tmpl, agentName, teamName string, env []string) string {
	// Expand template vars in the entire template first.
	expanded := strings.ReplaceAll(tmpl, "{{agent-name}}", agentName)
	expanded = strings.ReplaceAll(expanded, "{{team-name}}", teamName)

	// Build subprocess env: os.Environ() base + caller-supplied overrides.
	// We pass env to runCommand so it can merge at exec time.

	var b strings.Builder
	for _, line := range strings.Split(expanded, "\n") {
		if strings.HasPrefix(line, "$ ") {
			cmd := strings.TrimPrefix(line, "$ ")
			output := runCommand(cmd, env)
			if output == "" {
				continue
			}
			b.WriteString("--- ")
			b.WriteString(cmd)
			b.WriteString(" ---\n")
			b.WriteString(output)
			b.WriteString("\n\n")
		} else {
			b.WriteString(line)
			b.WriteString("\n")
		}
	}

	return strings.TrimRight(b.String(), "\n")
}

// runCommand executes a shell command with a timeout and returns the trimmed stdout output.
// env is merged into os.Environ() — later entries override earlier ones.
// Returns empty string when the command fails (non-zero exit or timeout).
// Failures are logged including stderr so callers can diagnose broken $ commands.
func runCommand(cmd string, env []string) string {
	ctx, cancel := context.WithTimeout(context.Background(), commandTimeout)
	defer cancel()

	//nolint:gosec // command comes from user config, not untrusted input
	c := exec.CommandContext(ctx, "sh", "-c", cmd)
	// Merge env: os.Environ() base, then overrides. Later entries win for duplicates.
	c.Env = append(os.Environ(), env...)

	var stderrBuf strings.Builder
	c.Stderr = &stderrBuf

	out, err := c.Output()
	if err != nil {
		if ctx.Err() == context.DeadlineExceeded {
			log.Printf("[promptrender] command timed out (skipped): %s", cmd)
		} else {
			stderr := strings.TrimRight(stderrBuf.String(), "\n")
			log.Printf("[promptrender] command failed (skipped): %s: %v\n%s", cmd, err, stderr)
		}
		return ""
	}
	return strings.TrimRight(string(out), "\n")
}
