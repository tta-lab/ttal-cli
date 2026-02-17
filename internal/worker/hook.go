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
	"time"

	"codeberg.org/clawteam/ttal-cli/internal/zellij"
)

// taskwarrior task status constants used across hook handlers.
const (
	taskStatusPending   = "pending"
	taskStatusCompleted = "completed"
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

func (t hookTask) ShortUUID() string {
	u := t.UUID()
	if len(u) > 8 {
		return u[:4] + "..." + u[len(u)-4:]
	}
	return u
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

func (t hookTask) Start() string {
	v, _ := t["start"].(string)
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
			tags = append(tags, s)
		}
	}
	return tags
}

func (t hookTask) HasTag(tag string) bool {
	for _, v := range t.Tags() {
		if v == tag {
			return true
		}
	}
	return false
}

func (t hookTask) Project() string {
	v, _ := t["project"].(string)
	return v
}

func (t hookTask) Annotations() []map[string]any {
	raw, ok := t["annotations"].([]any)
	if !ok {
		return nil
	}
	anns := make([]map[string]any, 0, len(raw))
	for _, v := range raw {
		if m, ok := v.(map[string]any); ok {
			anns = append(anns, m)
		}
	}
	return anns
}

// hookConfig holds config for hook delivery routing.
//
// Example config.json:
//
//	{
//	  "primary": "zellij",
//	  "channels": {
//	    "zellij": { "session": "cclaw", "tab": "kestrel" },
//	    "telegram": { "chat_id": "123", "account_id": "kestrel" }
//	  }
//	}
type hookConfig struct {
	Primary  string                   `json:"primary"`
	Channels map[string]channelConfig `json:"channels"`
}

// channelConfig holds channel-specific settings (union of all channel fields).
type channelConfig struct {
	// Zellij
	Session string `json:"session,omitempty"`
	Tab     string `json:"tab,omitempty"`
	DataDir string `json:"data_dir,omitempty"`

	// Telegram (via openclaw)
	ChatID    string `json:"chat_id,omitempty"`
	AccountID string `json:"account_id,omitempty"`
}

func loadHookConfig() (*hookConfig, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil, err
	}

	path := filepath.Join(home, ".task", "hooks", "config.json")
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("hook config not found: %s", path)
	}

	var cfg hookConfig
	if err := json.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("invalid hook config: %w", err)
	}

	if cfg.Primary == "" {
		return nil, fmt.Errorf("hook config missing 'primary' channel")
	}
	if cfg.Channels == nil {
		return nil, fmt.Errorf("hook config missing 'channels'")
	}
	if _, ok := cfg.Channels[cfg.Primary]; !ok {
		return nil, fmt.Errorf("primary channel %q not found in channels", cfg.Primary)
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

// notifyAgent sends a fire-and-forget message to the worker-lifecycle agent.
func notifyAgent(message string) {
	notifyAgentWith(message, "worker-lifecycle")
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

	// Daemon not running — fall back to direct zellij delivery via hook config
	cfg, err := loadHookConfig()
	if err != nil {
		hookLogFile("ERROR: cannot load config for agent notify: " + err.Error())
		return
	}

	ch := cfg.Channels[cfg.Primary]

	switch cfg.Primary {
	case "zellij":
		deliverViaZellij(message, ch)
	case "telegram":
		deliverViaTelegram(message, agent, ch)
	default:
		hookLogFile("ERROR: unknown channel: " + cfg.Primary)
	}
}

// deliverViaZellij sends the message as typed input to a CC session in a zellij tab.
func deliverViaZellij(message string, ch channelConfig) {
	if ch.Session == "" {
		hookLogFile("ERROR: zellij channel missing 'session'")
		return
	}

	if err := zellij.WriteChars(ch.Session, ch.Tab, ch.DataDir, message); err != nil {
		hookLogFile("ERROR: failed to deliver to zellij: " + err.Error())
		return
	}

	hookLogFile(fmt.Sprintf("Delivered to zellij session=%s tab=%s", ch.Session, ch.Tab))
}

// deliverViaTelegram sends the message via openclaw agent to Telegram.
func deliverViaTelegram(message, agent string, ch channelConfig) {
	if ch.ChatID == "" {
		hookLogFile("ERROR: telegram channel missing 'chat_id'")
		return
	}

	openclaw, err := exec.LookPath("openclaw")
	if err != nil {
		hookLogFile("ERROR: openclaw command not found")
		return
	}

	accountID := ch.AccountID
	if accountID == "" {
		accountID = "kestrel"
	}

	cmd := exec.Command(openclaw, "agent",
		"--message", message,
		"--agent", agent,
		"--deliver",
		"--channel", "telegram",
		"--reply-account", accountID,
		"--to", ch.ChatID,
	)
	cmd.Stdout = nil
	cmd.Stderr = nil

	if err := cmd.Start(); err != nil {
		hookLogFile("ERROR: failed to spawn openclaw: " + err.Error())
		return
	}

	// Fire-and-forget: release the child process
	go cmd.Wait() //nolint:errcheck
	hookLogFile(fmt.Sprintf("Agent notification spawned (pid=%d)", cmd.Process.Pid))
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

// extractTaskContext formats task info for agent notification.
func extractTaskContext(task hookTask) string {
	annotations := task.Annotations()
	var annTexts []string
	for _, ann := range annotations {
		if desc, ok := ann["description"].(string); ok {
			annTexts = append(annTexts, desc)
		}
	}
	annText := strings.Join(annTexts, "\n")
	if len(annText) > 2000 {
		annText = annText[:2000]
	}

	return fmt.Sprintf(`Task %s started: %s
Project: %s
Tags: %s

Annotations:
%s

Please analyze this task and spawn a worker if appropriate.
Derive worker name and project path from the context.`,
		task.UUID(), task.Description(),
		task.Project(), strings.Join(task.Tags(), ", "),
		annText)
}
