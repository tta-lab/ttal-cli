package zellij

import (
	"context"
	"fmt"
	"math/rand"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"time"
	"unicode"

	"codeberg.org/clawteam/ttal-cli/internal/env"
	"github.com/creack/pty"
)

const (
	cmdTimeout      = 10 * time.Second
	writeCharsDelay = 200 * time.Millisecond // delay between write-chars and Enter to let text render
)

// DataDir returns the zellij data directory.
func DataDir() string {
	if d := os.Getenv("TTAL_ZELLIJ_DATA_DIR"); d != "" {
		return d
	}
	tmpDir := os.Getenv("TMPDIR")
	if tmpDir == "" {
		tmpDir = "/tmp"
	}
	return filepath.Join(tmpDir, "ttal-zellij-data")
}

// WriteChars sends text to a zellij pane via write-chars, then sends Enter via
// a separate `write 10` command (raw byte). This is the only reliable way to
// trigger Enter — embedding `\n` in write-chars does not send Enter.
//
// The text is sanitized before sending: newlines and carriage returns are
// replaced with spaces, and other control characters are stripped. This
// prevents shell injection via external input (e.g. Telegram messages).
func WriteChars(session, tab, dataDir, text string) error {
	zellijBin, err := exec.LookPath("zellij")
	if err != nil {
		return fmt.Errorf("zellij not found in PATH")
	}

	baseArgs := []string{}
	if dataDir != "" {
		baseArgs = append(baseArgs, "--data-dir", dataDir)
	}
	baseArgs = append(baseArgs, "--session", session, "action")

	// Focus target tab if specified
	if tab != "" {
		focusArgs := append(append([]string{}, baseArgs...), "go-to-tab-name", tab)
		cmd := exec.Command(zellijBin, focusArgs...)
		if out, err := cmd.CombinedOutput(); err != nil {
			return fmt.Errorf("failed to focus tab %q: %w: %s", tab, err, strings.TrimSpace(string(out)))
		}
	}

	// Sanitize: replace newlines/CR with space, strip other control chars
	safe := sanitizeForTerminal(text)

	// Write sanitized text
	writeArgs := append(append([]string{}, baseArgs...), "write-chars", safe)
	cmd := exec.Command(zellijBin, writeArgs...)
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("write-chars failed: %w: %s", err, strings.TrimSpace(string(out)))
	}

	time.Sleep(writeCharsDelay)

	// Send Enter as raw byte 13 (CR) — terminals use CR to submit input
	enterArgs := append(append([]string{}, baseArgs...), "write", "13")
	cmd = exec.Command(zellijBin, enterArgs...)
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("write Enter failed: %w: %s", err, strings.TrimSpace(string(out)))
	}

	return nil
}

// sanitizeForTerminal replaces newlines/CR with spaces and strips other
// control characters to prevent injection via external input.
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

// SessionExists checks if a zellij session with the given name exists.
func SessionExists(name string) bool {
	sessions, err := ListSessions()
	if err != nil {
		return false
	}
	for _, s := range sessions {
		if s == name {
			return true
		}
	}
	return false
}

// KillSession sends a kill signal to a zellij session, terminating all its processes.
func KillSession(name string) error {
	ctx, cancel := context.WithTimeout(context.Background(), cmdTimeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, "zellij", "--data-dir", DataDir(), "kill-session", name)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to kill session %s: %w: %s", name, err, strings.TrimSpace(string(out)))
	}
	return nil
}

// DeleteSession deletes an exited zellij session.
func DeleteSession(name string) error {
	ctx, cancel := context.WithTimeout(context.Background(), cmdTimeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, "zellij", "--data-dir", DataDir(), "delete-session", name)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to delete session %s: %w: %s", name, err, strings.TrimSpace(string(out)))
	}
	return nil
}

// ListSessions returns the names of all active zellij sessions.
func ListSessions() ([]string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), cmdTimeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, "zellij", "--data-dir", DataDir(), "list-sessions", "--short")
	out, err := cmd.CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("failed to list sessions: %w", err)
	}

	var sessions []string
	for _, line := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		if name := strings.TrimSpace(line); name != "" {
			sessions = append(sessions, name)
		}
	}
	return sessions, nil
}

// GenerateSessionID creates a random 8-character alphanumeric session ID.
func GenerateSessionID() string {
	const chars = "abcdefghijklmnopqrstuvwxyz0123456789"
	b := make([]byte, 8)
	for i := range b {
		b[i] = chars[rand.Intn(len(chars))]
	}
	return string(b)
}

// LayoutConfig holds configuration for creating a zellij layout.
type LayoutConfig struct {
	WorkDir        string
	Task           string
	Yolo           bool
	Brainstorm     bool
	Model          string
	Branch         string
	IsWorktree     bool
	GatekeeperPath string
}

// CreateLayout writes a KDL layout file and task file to temp directory.
// Returns the paths to the layout file and task file.
func CreateLayout(cfg LayoutConfig) (layoutPath, taskFilePath string, err error) {
	// Build the full task text with worktree/brainstorm prefixes
	fullTask := cfg.Task
	if cfg.IsWorktree && cfg.Branch != "" {
		worktreePrefix := fmt.Sprintf(`IMPORTANT - You are in a git worktree:
- Working directory: %s
- Branch: %s
- This is an isolated workspace for your task
- STAY in this directory - do NOT cd to parent/main workspace
- All your work should happen here
- When done: Use the /create-pr skill to finalize (commits, pushes, creates PR, updates task)

Your task:

`, cfg.WorkDir, cfg.Branch)
		fullTask = worktreePrefix + fullTask
	}

	if cfg.Brainstorm {
		brainstormPrefix := `Use the brainstorming skill before implementation:

1. Understand the project context (check files, docs, recent commits)
2. Ask clarifying questions one at a time to refine requirements
3. Explore different approaches with trade-offs
4. Present the design in sections, validating each part
5. Document the design in docs/plans/YYYY-MM-DD-<topic>-design.md

Then proceed with:

`
		fullTask = brainstormPrefix + fullTask
	}

	// Write task to temp file
	taskFile, err := os.CreateTemp("", "claude-task-*.txt")
	if err != nil {
		return "", "", fmt.Errorf("failed to create task file: %w", err)
	}
	if _, err := taskFile.WriteString(fullTask); err != nil {
		_ = taskFile.Close()
		return "", "", fmt.Errorf("failed to write task file: %w", err)
	}
	_ = taskFile.Close()
	taskFilePath = taskFile.Name()

	// Build fish command
	yoloFlag := ""
	if cfg.Yolo {
		yoloFlag = "--dangerously-skip-permissions "
	}

	if cfg.GatekeeperPath == "" {
		return "", "", fmt.Errorf("gatekeeper path is required")
	}

	model := cfg.Model
	if model == "" {
		model = "opus"
	}

	fishCommand := fmt.Sprintf(
		"%s --task-file %s -- claude --model %s %s--",
		cfg.GatekeeperPath, taskFilePath, model, yoloFlag)

	// Create layout file
	layoutContent := fmt.Sprintf(`layout {
    tab name="worker" focus=true {
        pane size=1 borderless=true {
            plugin location="tab-bar"
        }

        pane {
            cwd "%s"
            command "fish"
            args "-C" "%s"
        }

        pane size=2 borderless=true {
            plugin location="status-bar"
        }
    }

    tab name="term" {
        pane size=1 borderless=true {
            plugin location="tab-bar"
        }

        pane {
            cwd "%s"
            command "fish"
            args "-C" "set -q TERM; or set -x TERM xterm-256color"
        }

        pane size=2 borderless=true {
            plugin location="status-bar"
        }
    }
}
`, cfg.WorkDir, fishCommand, cfg.WorkDir)

	layoutFile, err := os.CreateTemp("", "zellij-layout-*.kdl")
	if err != nil {
		return "", "", fmt.Errorf("failed to create layout file: %w", err)
	}
	if _, err := layoutFile.WriteString(layoutContent); err != nil {
		_ = layoutFile.Close()
		return "", "", fmt.Errorf("failed to write layout file: %w", err)
	}
	_ = layoutFile.Close()
	layoutPath = layoutFile.Name()

	return layoutPath, taskFilePath, nil
}

// SessionHandle holds the process and PTY of a launched zellij session.
type SessionHandle struct {
	Process *os.Process
	ptmx    *os.File
}

// Close releases the PTY file descriptor.
func (h *SessionHandle) Close() {
	if h.ptmx != nil {
		_ = h.ptmx.Close()
		h.ptmx = nil
	}
}

// LaunchSession starts a zellij session with the given layout.
// The caller must call handle.Close() after WaitForSession succeeds to release the PTY.
func LaunchSession(name, layoutFile string) (*SessionHandle, error) {
	cmd := exec.Command("zellij",
		"--data-dir", DataDir(),
		"--new-session-with-layout", layoutFile,
		"--session", name,
	)
	cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true}

	// Strip CLAUDECODE env var so the spawned CC session doesn't think it's nested
	cmd.Env = env.ForSpawnCC()

	// Start with a PTY so zellij-client can query terminal attributes
	ptmx, err := pty.Start(cmd)
	if err != nil {
		return nil, fmt.Errorf("failed to start zellij with PTY: %w", err)
	}

	return &SessionHandle{Process: cmd.Process, ptmx: ptmx}, nil
}

// WaitForSession polls until the session appears in zellij's session list.
// On success, closes the PTY handle since zellij server runs independently.
func WaitForSession(name string, handle *SessionHandle, timeout time.Duration) error {
	defer handle.Close()

	start := time.Now()
	checkInterval := 500 * time.Millisecond

	for time.Since(start) < timeout {
		time.Sleep(checkInterval)

		// Check if process has crashed
		var status syscall.WaitStatus
		pid, err := syscall.Wait4(handle.Process.Pid, &status, syscall.WNOHANG, nil)
		if err == nil && pid != 0 {
			return fmt.Errorf("zellij process exited with code %d", status.ExitStatus())
		}

		if SessionExists(name) {
			return nil
		}
	}

	return fmt.Errorf("session creation timed out after %s", timeout)
}

// DumpSessionState captures the current state of a worker's worktree for debugging.
// Returns the path to the dump file.
func DumpSessionState(sessionName, workDir, workerName string) (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("failed to get home directory: %w", err)
	}

	dumpDir := filepath.Join(home, ".ttal", "dumps")
	if err := os.MkdirAll(dumpDir, 0o755); err != nil {
		return "", fmt.Errorf("failed to create dump directory: %w", err)
	}

	timestamp := time.Now().Format("2006-01-02_15-04-05")
	dumpFile := filepath.Join(dumpDir, fmt.Sprintf("%s_%s.txt", workerName, timestamp))

	var sections []string
	sections = append(sections, fmt.Sprintf("Worker State Dump: %s", workerName))
	sections = append(sections, fmt.Sprintf("Timestamp: %s", timestamp))
	sections = append(sections, fmt.Sprintf("Session: %s", sessionName))
	sections = append(sections, fmt.Sprintf("Work dir: %s", workDir))
	sections = append(sections, "")

	// Git log
	if out, err := runGit(workDir, "log", "--oneline", "-20"); err == nil {
		sections = append(sections, "=== Recent commits ===", out)
	}

	// Git status
	if out, err := runGit(workDir, "status", "--short"); err == nil {
		sections = append(sections, "=== Git status ===", out)
	}

	// Commits not in default branch
	if out, err := runGit(workDir, "log", "--oneline", "main..HEAD"); err == nil && strings.TrimSpace(out) != "" {
		sections = append(sections, "=== Commits not in main ===", out)
	}

	content := strings.Join(sections, "\n")
	if err := os.WriteFile(dumpFile, []byte(content), 0o644); err != nil {
		return "", fmt.Errorf("failed to write dump file: %w", err)
	}

	return dumpFile, nil
}

// CleanupWorker removes the zellij session, git worktree, and worker branch.
func CleanupWorker(sessionName, workDir, branch, projectDir string) error {
	// Kill then delete zellij session
	if SessionExists(sessionName) {
		if err := KillSession(sessionName); err != nil {
			return fmt.Errorf("failed to kill session: %w", err)
		}

		// Wait for session processes to exit before deleting
		for range 15 {
			time.Sleep(200 * time.Millisecond)
			if !SessionExists(sessionName) {
				break
			}
		}

		// Delete the now-exited session
		if SessionExists(sessionName) {
			if err := DeleteSession(sessionName); err != nil {
				return fmt.Errorf("failed to delete session: %w", err)
			}
		}
	}

	// Remove git worktree (must happen before branch deletion)
	if _, err := os.Stat(workDir); err == nil {
		if _, err := runGit(projectDir, "worktree", "remove", workDir, "--force"); err != nil {
			return fmt.Errorf("failed to remove worktree: %w", err)
		}
	}

	// Delete the worker branch
	if branch != "" {
		if _, err := runGit(projectDir, "branch", "-D", branch); err != nil {
			// Non-fatal: branch may already be deleted
			fmt.Fprintf(os.Stderr, "warning: failed to delete branch %s: %v\n", branch, err)
		}
	}

	return nil
}

// CheckWorktreeClean returns true if the worktree has no uncommitted changes.
func CheckWorktreeClean(workDir string) bool {
	out, err := runGit(workDir, "status", "--porcelain")
	if err != nil {
		return false
	}
	return strings.TrimSpace(out) == ""
}

func runGit(dir string, args ...string) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), cmdTimeout)
	defer cancel()

	fullArgs := append([]string{"-C", dir}, args...)
	cmd := exec.CommandContext(ctx, "git", fullArgs...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("%w: %s", err, strings.TrimSpace(string(out)))
	}
	return string(out), nil
}

