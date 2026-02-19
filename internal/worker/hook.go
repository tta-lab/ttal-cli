package worker

import (
	"bufio"
	"encoding/json"
	"fmt"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"codeberg.org/clawteam/ttal-cli/internal/zellij"
)

// taskwarrior task status constants used across hook handlers.
const (
	taskStatusPending      = "pending"
	taskStatusCompleted    = "completed"
	defaultLifecycleAgent  = "kestrel"
)

// daemonSendRequest mirrors daemon.SendRequest to avoid import cycle.
type daemonSendRequest struct {
	From    string `json:"from,omitempty"`
	To      string `json:"to,omitempty"`
	Message string `json:"message"`
}

type daemonSendResponse struct {
	OK    bool   `json:"ok"`
	Error string `json:"error,omitempty"`
}

// errDaemonNotRunning is returned when the daemon socket cannot be reached.
// Callers use this to distinguish "daemon down, fall back" from
// "daemon up but rejected the request".
var errDaemonNotRunning = fmt.Errorf("daemon not running")

// sendToDaemon sends a request to the daemon socket.
// Returns errDaemonNotRunning if the socket is unreachable.
// Returns a descriptive error (prefixed "daemon error:") if the daemon
// accepted the connection but rejected the request.
func sendToDaemon(req daemonSendRequest) error {
	home, err := os.UserHomeDir()
	if err != nil {
		return errDaemonNotRunning
	}
	sockPath := filepath.Join(home, ".ttal", "daemon.sock")

	conn, err := net.DialTimeout("unix", sockPath, 5*time.Second)
	if err != nil {
		return errDaemonNotRunning
	}
	defer conn.Close()                                //nolint:errcheck
	conn.SetDeadline(time.Now().Add(5 * time.Second)) //nolint:errcheck

	data, _ := json.Marshal(req)
	if _, err := conn.Write(append(data, '\n')); err != nil {
		return fmt.Errorf("failed to send to daemon: %w", err)
	}

	scanner := bufio.NewScanner(conn)
	if !scanner.Scan() {
		return fmt.Errorf("no response from daemon")
	}

	var resp daemonSendResponse
	if err := json.Unmarshal(scanner.Bytes(), &resp); err != nil {
		return err
	}
	if !resp.OK {
		return fmt.Errorf("daemon error: %s", resp.Error)
	}
	return nil
}

// hookTask represents a taskwarrior task as received via on-modify hook stdin.
// Uses map for flexibility — we only inspect specific fields.
type hookTask map[string]any

func (t hookTask) UUID() string {
	v, _ := t["uuid"].(string)
	return v
}

func (t hookTask) Description() string {
	v, _ := t["description"].(string)
	return v
}

func (t hookTask) Status() string {
	v, _ := t["status"].(string)
	return v
}

func (t hookTask) SessionName() string {
	v, _ := t["session_name"].(string)
	return v
}

func (t hookTask) Tags() []string {
	raw, ok := t["tags"].([]any)
	if !ok {
		return nil
	}
	tags := make([]string, 0, len(raw))
	for _, v := range raw {
		if s, ok := v.(string); ok {
			tags = append(tags, strings.ToLower(s))
		}
	}
	return tags
}

func (t hookTask) ProjectPath() string {
	v, _ := t["project_path"].(string)
	return v
}

func (t hookTask) Branch() string {
	v, _ := t["branch"].(string)
	return v
}

func (t hookTask) Start() string {
	v, _ := t["start"].(string)
	return v
}

// hookFallbackConfig is a minimal reader for ~/.ttal/daemon.json used when the
// daemon is not running and for resolving the lifecycle agent name.
type hookFallbackConfig struct {
	ZellijSession  string `json:"zellij_session"`
	LifecycleAgent string `json:"lifecycle_agent"`
}

func loadHookFallbackConfig() (*hookFallbackConfig, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil, err
	}

	path := filepath.Join(home, ".ttal", "daemon.json")
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("daemon config not found: %s", path)
	}

	var cfg hookFallbackConfig
	if err := json.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("invalid daemon config: %w", err)
	}

	if cfg.ZellijSession == "" {
		return nil, fmt.Errorf("daemon config missing 'zellij_session'")
	}

	return &cfg, nil
}

// readHookInput reads original and modified task JSON from stdin (taskwarrior on-modify protocol).
func readHookInput() (original, modified hookTask, err error) {
	scanner := bufio.NewScanner(os.Stdin)
	scanner.Buffer(make([]byte, 0, 1024*1024), 1024*1024)

	if !scanner.Scan() {
		if err := scanner.Err(); err != nil {
			return nil, nil, fmt.Errorf("reading original task from stdin: %w", err)
		}
		return nil, nil, fmt.Errorf("failed to read original task from stdin")
	}
	if err := json.Unmarshal(scanner.Bytes(), &original); err != nil {
		return nil, nil, fmt.Errorf("failed to parse original task: %w", err)
	}

	if !scanner.Scan() {
		if err := scanner.Err(); err != nil {
			return nil, nil, fmt.Errorf("reading modified task from stdin: %w", err)
		}
		return nil, nil, fmt.Errorf("failed to read modified task from stdin")
	}
	if err := json.Unmarshal(scanner.Bytes(), &modified); err != nil {
		return nil, nil, fmt.Errorf("failed to parse modified task: %w", err)
	}

	return original, modified, nil
}

// readHookAddInput reads a single task JSON from stdin (taskwarrior on-add protocol).
// On success returns the parsed task. On failure returns the raw line so the caller
// can echo it back to stdout (taskwarrior drops the task if nothing is written).
func readHookAddInput() (task hookTask, rawLine []byte, err error) {
	scanner := bufio.NewScanner(os.Stdin)
	scanner.Buffer(make([]byte, 0, 1024*1024), 1024*1024)

	if !scanner.Scan() {
		if err := scanner.Err(); err != nil {
			return nil, nil, fmt.Errorf("reading task from stdin: %w", err)
		}
		return nil, nil, fmt.Errorf("failed to read task from stdin")
	}

	rawLine = append([]byte{}, scanner.Bytes()...)

	if err := json.Unmarshal(rawLine, &task); err != nil {
		return nil, rawLine, fmt.Errorf("failed to parse task: %w", err)
	}

	return task, rawLine, nil
}

// forkBackground launches a detached subprocess that runs independently of the hook process.
// Used for fire-and-forget operations that must not block taskwarrior.
func forkBackground(args ...string) error {
	ttalBin, err := exec.LookPath("ttal")
	if err != nil {
		return fmt.Errorf("ttal not found in PATH: %w", err)
	}

	cmd := exec.Command(ttalBin, args...)
	cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
	cmd.Stdout = nil
	cmd.Stderr = nil
	cmd.Stdin = nil

	if err := cmd.Start(); err != nil {
		return fmt.Errorf("failed to fork background process: %w", err)
	}

	// Child is in its own session (Setsid), so it gets reparented to PID 1
	// on hook exit. PID 1 (launchd) reaps it — no Wait() needed.
	return nil
}

// passthroughTask writes the task JSON back to stdout as required by the
// taskwarrior hook protocol. We never mutate the task — this is a pure echo.
// Marshal of a JSON-sourced map[string]any cannot fail.
func passthroughTask(task hookTask) {
	data, _ := json.Marshal(task)
	fmt.Println(string(data))
}

// hookLog writes a structured log line to ~/.task/hooks.log.
func hookLog(eventType, taskUUID, description string, kvs ...string) {
	home, err := os.UserHomeDir()
	if err != nil {
		return
	}

	logPath := filepath.Join(home, ".task", "hooks.log")
	timestamp := time.Now().Format("15:04:05")

	shortUUID := taskUUID
	if len(taskUUID) > 8 {
		shortUUID = taskUUID[:4] + "..." + taskUUID[len(taskUUID)-4:]
	}

	parts := []string{fmt.Sprintf("task=%s", shortUUID)}
	for i := 0; i+1 < len(kvs); i += 2 {
		parts = append(parts, fmt.Sprintf("%s=%s", kvs[i], kvs[i+1]))
	}

	line := fmt.Sprintf("[%s] %-12s | %s | %s\n", timestamp, eventType, strings.Join(parts, " "), description)

	f, err := os.OpenFile(logPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return
	}
	defer f.Close()     //nolint:errcheck
	f.WriteString(line) //nolint:errcheck
}

// notifyAgent sends a fire-and-forget message to the lifecycle agent.
// Resolves the agent name from daemon.json lifecycle_agent field.
func notifyAgent(message string) {
	agent := defaultLifecycleAgent
	if cfg, err := loadHookFallbackConfig(); err == nil && cfg.LifecycleAgent != "" {
		agent = cfg.LifecycleAgent
	}
	notifyAgentWith(message, agent)
}

// notifyTelegram sends a message to an agent's Telegram chat via the daemon.
// Uses From-only routing (daemon's handleFrom → Telegram Bot API).
// Fire-and-forget: errors are logged but not propagated.
func notifyTelegram(message string) {
	agent := defaultLifecycleAgent
	if cfg, err := loadHookFallbackConfig(); err == nil && cfg.LifecycleAgent != "" {
		agent = cfg.LifecycleAgent
	}
	req := daemonSendRequest{From: agent, Message: message}
	if err := sendToDaemon(req); err != nil {
		hookLogFile(fmt.Sprintf("ERROR: telegram notify failed for %s: %v", agent, err))
	}
}

// notifyAgentWith dispatches a message to the named agent.
// Tries the daemon socket first; falls back to direct delivery only if the
// daemon is not running. A daemon rejection (unknown agent, etc.) is logged
// and does not trigger fallback.
func notifyAgentWith(message, agent string) {
	// Try daemon socket first
	req := daemonSendRequest{To: agent, Message: message}
	err := sendToDaemon(req)
	if err == nil {
		hookLogFile(fmt.Sprintf("Delivered via daemon: agent=%s", agent))
		return
	}
	if err != errDaemonNotRunning {
		// Daemon is running but rejected the request — don't fall back silently
		hookLogFile(fmt.Sprintf("ERROR: daemon rejected notify for agent=%s: %v", agent, err))
		return
	}

	// Daemon not running — fall back to direct zellij delivery
	cfg, err := loadHookFallbackConfig()
	if err != nil {
		hookLogFile("ERROR: cannot load fallback config: " + err.Error())
		return
	}

	if err := zellij.WriteChars(cfg.ZellijSession, agent, "", message); err != nil {
		hookLogFile("ERROR: failed to deliver to zellij: " + err.Error())
		return
	}

	hookLogFile(fmt.Sprintf("Delivered via zellij fallback: session=%s tab=%s", cfg.ZellijSession, agent))
}

func hookLogFile(message string) {
	home, err := os.UserHomeDir()
	if err != nil {
		return
	}
	logPath := filepath.Join(home, ".task", "hooks.log")
	timestamp := time.Now().Format(time.RFC3339)
	f, err := os.OpenFile(logPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return
	}
	defer f.Close()                                 //nolint:errcheck
	fmt.Fprintf(f, "[%s] %s\n", timestamp, message) //nolint:errcheck
}

