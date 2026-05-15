package tmux

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"
	"unicode"
)

const (
	cmdTimeout    = 10 * time.Second
	sendKeysDelay = 500 * time.Millisecond
)

// IsolatedEnv returns the process environment for TTAL-owned tmux commands.
//
// TTAL uses one tmux socket namespace for daemon and CLI commands. Ambient
// TMUX and TMUX_TMPDIR are ignored so user tmux sessions cannot pick TTAL's
// namespace. Set TTAL_TMUX_TMPDIR to override the TTAL socket directory
// deliberately.
func IsolatedEnv() []string {
	dir := defaultTmpDir()
	if dir == "" {
		return os.Environ()
	}
	_ = os.MkdirAll(dir, 0o700)
	return append(envWithout("TMUX", "TMUX_TMPDIR"), "TMUX_TMPDIR="+dir)
}

func defaultTmpDir() string {
	if override := os.Getenv("TTAL_TMUX_TMPDIR"); override != "" {
		return override
	}
	if runtimeDir := os.Getenv("XDG_RUNTIME_DIR"); runtimeDir != "" {
		return filepath.Join(runtimeDir, "ttal-tmux")
	}
	return filepath.Join(os.TempDir(), "ttal-tmux-"+strconv.Itoa(os.Getuid()))
}

func commandContext(ctx context.Context, args ...string) *exec.Cmd {
	cmd := exec.CommandContext(ctx, "tmux", args...)
	cmd.Env = IsolatedEnv()
	return cmd
}

func envWithout(keys ...string) []string {
	drop := make(map[string]bool, len(keys))
	for _, key := range keys {
		drop[key+"="] = true
	}

	var env []string
	for _, part := range os.Environ() {
		keep := true
		for prefix := range drop {
			if strings.HasPrefix(part, prefix) {
				keep = false
				break
			}
		}
		if keep {
			env = append(env, part)
		}
	}
	return env
}

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
	cmd := commandContext(ctx, "send-keys", "-l", "-t", target, safe)
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("send-keys failed: %w: %s", err, strings.TrimSpace(string(out)))
	}

	time.Sleep(sendKeysDelay)

	ctx2, cancel2 := context.WithTimeout(context.Background(), cmdTimeout)
	defer cancel2()
	cmd = commandContext(ctx2, "send-keys", "-t", target, "Enter")
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("send-keys Enter failed: %w: %s", err, strings.TrimSpace(string(out)))
	}

	return nil
}

// SessionExists checks if a tmux session exists (exact match).
func SessionExists(name string) bool {
	ctx, cancel := context.WithTimeout(context.Background(), cmdTimeout)
	defer cancel()
	cmd := commandContext(ctx, "has-session", "-t", "="+name)
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

	cmd := commandContext(ctx, args...)
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

	cmd := commandContext(ctx, args...)
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("new-window failed: %w: %s", err, strings.TrimSpace(string(out)))
	}
	return nil
}

// WindowExists checks if a named window exists in a session.
func WindowExists(session, window string) bool {
	ctx, cancel := context.WithTimeout(context.Background(), cmdTimeout)
	defer cancel()

	cmd := commandContext(ctx, "list-windows", "-t", session, "-F", "#{window_name}")
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

// FirstWindow returns the name of the first window in a session.
// Returns "" with nil error if the session has no windows.
func FirstWindow(session string) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), cmdTimeout)
	defer cancel()

	cmd := commandContext(ctx, "list-windows", "-t", session, "-F", "#{window_name}")
	out, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("tmux list-windows for %q: %w", session, err)
	}
	lines := strings.Split(strings.TrimSpace(string(out)), "\n")
	if len(lines) == 0 || lines[0] == "" {
		return "", nil
	}
	return strings.TrimSpace(lines[0]), nil
}

// KillWindow kills a specific window in a session.
func KillWindow(session, window string) error {
	ctx, cancel := context.WithTimeout(context.Background(), cmdTimeout)
	defer cancel()

	cmd := commandContext(ctx, "kill-window", "-t", session+":"+window)
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("kill-window failed: %w: %s", err, strings.TrimSpace(string(out)))
	}
	return nil
}

// KillSession kills a tmux session.
func KillSession(name string) error {
	ctx, cancel := context.WithTimeout(context.Background(), cmdTimeout)
	defer cancel()

	cmd := commandContext(ctx, "kill-session", "-t", "="+name)
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("kill-session failed: %w: %s", err, strings.TrimSpace(string(out)))
	}
	return nil
}

// ListSessions returns names of all tmux sessions.
func ListSessions() ([]string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), cmdTimeout)
	defer cancel()

	cmd := commandContext(ctx, "list-sessions", "-F", "#{session_name}")
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
	cmd := commandContext(ctx, "send-keys", "-t", target, key)
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

	cmd := commandContext(ctx, "set-environment", "-t", session, key, value)
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("set-environment failed: %w: %s", err, strings.TrimSpace(string(out)))
	}
	return nil
}

// CurrentSession returns the name of the ambient tmux session this process is running in.
// It deliberately uses the caller's TMUX environment instead of TTAL's isolated
// server, because it answers "where is this command running now?"
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

// CurrentWindow returns the name of the ambient tmux window this process is running in.
// It deliberately uses the caller's TMUX environment instead of TTAL's isolated
// server, because it answers "where is this command running now?"
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

	cmd := commandContext(ctx, "display-message", "-t", target, "-p", "#{pane_current_path}")
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

	cmd := commandContext(ctx, args...)
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
