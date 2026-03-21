package tmux

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"
	"unicode"
)

const (
	cmdTimeout    = 10 * time.Second
	sendKeysDelay = 500 * time.Millisecond
)

// SendKeys sends text to a tmux pane, then sends Enter.
// target format: "session:window" or "session:window.pane"
// Text is sanitized before sending.
func SendKeys(session, window, text string) error {
	target := session
	if window != "" {
		target = session + ":" + window
	}

	safe := sanitizeForTerminal(text)

	ctx, cancel := context.WithTimeout(context.Background(), cmdTimeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, "tmux", "send-keys", "-l", "-t", target, safe)
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("send-keys failed: %w: %s", err, strings.TrimSpace(string(out)))
	}

	time.Sleep(sendKeysDelay)

	ctx2, cancel2 := context.WithTimeout(context.Background(), cmdTimeout)
	defer cancel2()
	cmd = exec.CommandContext(ctx2, "tmux", "send-keys", "-t", target, "Enter")
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("send-keys Enter failed: %w: %s", err, strings.TrimSpace(string(out)))
	}

	return nil
}

// SessionExists checks if a tmux session exists (exact match).
func SessionExists(name string) bool {
	ctx, cancel := context.WithTimeout(context.Background(), cmdTimeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, "tmux", "has-session", "-t", "="+name)
	return cmd.Run() == nil
}

// NewSession creates a new detached tmux session.
// window is the name for the first window. command is run in that window.
func NewSession(session, window, workDir, command string) error {
	ctx, cancel := context.WithTimeout(context.Background(), cmdTimeout)
	defer cancel()

	args := []string{"new-session", "-d", "-s", session, "-n", window, "-c", workDir}
	if command != "" {
		args = append(args, command)
	}

	cmd := exec.CommandContext(ctx, "tmux", args...)
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("new-session failed: %w: %s", err, strings.TrimSpace(string(out)))
	}
	return nil
}

// NewWindow adds a window to an existing session.
func NewWindow(session, window, workDir, command string) error {
	ctx, cancel := context.WithTimeout(context.Background(), cmdTimeout)
	defer cancel()

	args := []string{"new-window", "-t", session, "-n", window, "-c", workDir}
	if command != "" {
		args = append(args, command)
	}

	cmd := exec.CommandContext(ctx, "tmux", args...)
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("new-window failed: %w: %s", err, strings.TrimSpace(string(out)))
	}
	return nil
}

// WindowExists checks if a named window exists in a session.
func WindowExists(session, window string) bool {
	ctx, cancel := context.WithTimeout(context.Background(), cmdTimeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, "tmux", "list-windows", "-t", session, "-F", "#{window_name}")
	out, err := cmd.CombinedOutput()
	if err != nil {
		return false
	}

	for _, line := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		if strings.TrimSpace(line) == window {
			return true
		}
	}
	return false
}

// FirstWindowExcept returns the first window in a session whose name is not in the
// exclusion list. Returns "" with nil error if no matching window is found.
func FirstWindowExcept(session string, exclude ...string) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), cmdTimeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, "tmux", "list-windows", "-t", session, "-F", "#{window_name}")
	out, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("tmux list-windows for %q: %w", session, err)
	}

	skip := make(map[string]bool, len(exclude))
	for _, e := range exclude {
		skip[e] = true
	}

	for _, line := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		name := strings.TrimSpace(line)
		if name != "" && !skip[name] {
			return name, nil
		}
	}
	return "", nil
}

// KillWindow kills a specific window in a session.
func KillWindow(session, window string) error {
	ctx, cancel := context.WithTimeout(context.Background(), cmdTimeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, "tmux", "kill-window", "-t", session+":"+window)
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("kill-window failed: %w: %s", err, strings.TrimSpace(string(out)))
	}
	return nil
}

// KillSession kills a tmux session.
func KillSession(name string) error {
	ctx, cancel := context.WithTimeout(context.Background(), cmdTimeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, "tmux", "kill-session", "-t", "="+name)
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("kill-session failed: %w: %s", err, strings.TrimSpace(string(out)))
	}
	return nil
}

// ListSessions returns names of all tmux sessions.
func ListSessions() ([]string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), cmdTimeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, "tmux", "list-sessions", "-F", "#{session_name}")
	out, err := cmd.CombinedOutput()
	if err != nil {
		if strings.Contains(string(out), "no server running") {
			return nil, nil
		}
		return nil, fmt.Errorf("list-sessions failed: %w: %s", err, strings.TrimSpace(string(out)))
	}

	var sessions []string
	for _, line := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		if name := strings.TrimSpace(line); name != "" {
			sessions = append(sessions, name)
		}
	}
	return sessions, nil
}

// SendRawKey sends a special key (e.g. "Escape", "C-c") to a tmux pane
// without the -l (literal) flag. Used for control keys that aren't text.
func SendRawKey(session, window, key string) error {
	target := session
	if window != "" {
		target = session + ":" + window
	}

	ctx, cancel := context.WithTimeout(context.Background(), cmdTimeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, "tmux", "send-keys", "-t", target, key)
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("send-keys failed: %w: %s", err, strings.TrimSpace(string(out)))
	}
	return nil
}

// SetEnv sets an environment variable on a tmux session.
// New processes spawned in the session will inherit this variable.
func SetEnv(session, key, value string) error {
	ctx, cancel := context.WithTimeout(context.Background(), cmdTimeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, "tmux", "set-environment", "-t", session, key, value)
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("set-environment failed: %w: %s", err, strings.TrimSpace(string(out)))
	}
	return nil
}

// CurrentSession returns the name of the tmux session this process is running in.
// Returns ("", nil) if not inside tmux. Returns an error if tmux command fails.
func CurrentSession() (string, error) {
	if os.Getenv("TMUX") == "" {
		return "", nil
	}

	ctx, cancel := context.WithTimeout(context.Background(), cmdTimeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, "tmux", "display-message", "-p", "#{session_name}")
	out, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("failed to get session name: %w: %s", err, strings.TrimSpace(string(out)))
	}
	return strings.TrimSpace(string(out)), nil
}

// CurrentWindow returns the name of the tmux window this process is running in.
// Uses TMUX_PANE to target the actual pane, not the active window — without -t,
// display-message returns whichever window the user is looking at.
// Returns ("", nil) if not inside tmux.
func CurrentWindow() (string, error) {
	if os.Getenv("TMUX") == "" {
		return "", nil
	}

	ctx, cancel := context.WithTimeout(context.Background(), cmdTimeout)
	defer cancel()

	args := []string{"display-message"}
	if pane := os.Getenv("TMUX_PANE"); pane != "" {
		args = append(args, "-t", pane)
	}
	args = append(args, "-p", "#{window_name}")

	cmd := exec.CommandContext(ctx, "tmux", args...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("failed to get window name: %w: %s", err, strings.TrimSpace(string(out)))
	}
	return strings.TrimSpace(string(out)), nil
}

// GetPaneCwd returns the current working directory of the pane in the given session:window.
func GetPaneCwd(session, window string) (string, error) {
	target := session
	if window != "" {
		target = session + ":" + window
	}

	ctx, cancel := context.WithTimeout(context.Background(), cmdTimeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, "tmux", "display-message", "-t", target, "-p", "#{pane_current_path}")
	out, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("get pane cwd: %w: %s", err, strings.TrimSpace(string(out)))
	}
	return strings.TrimSpace(string(out)), nil
}

// RespawnWindow kills any existing process and starts a new command in the window.
func RespawnWindow(session, window, workDir, command string) error {
	ctx, cancel := context.WithTimeout(context.Background(), cmdTimeout)
	defer cancel()

	target := session + ":" + window
	args := []string{"respawn-window", "-k", "-t", target}
	if workDir != "" {
		args = append(args, "-c", workDir)
	}
	args = append(args, command)

	cmd := exec.CommandContext(ctx, "tmux", args...)
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("respawn-window failed: %w: %s", err, strings.TrimSpace(string(out)))
	}
	return nil
}

// sanitizeForTerminal replaces newlines/CR with spaces and strips control chars.
func sanitizeForTerminal(s string) string {
	var b strings.Builder
	for _, r := range s {
		switch {
		case r == '\n' || r == '\r':
			b.WriteRune(' ')
		case unicode.IsControl(r):
			// strip DEL, ESC, and other control chars
		default:
			b.WriteRune(r)
		}
	}
	return b.String()
}
