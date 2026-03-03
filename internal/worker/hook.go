package worker

import (
	"bufio"
	"encoding/json"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/tta-lab/ttal-cli/internal/config"
	"github.com/tta-lab/ttal-cli/internal/notify"
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
	Team    string `json:"team,omitempty"`
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
	if req.Team == "" {
		req.Team = os.Getenv("TTAL_TEAM")
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("user home dir: %w", err)
	}
	sockPath := filepath.Join(home, ".ttal", "daemon.sock")

	conn, err := net.DialTimeout("unix", sockPath, 5*time.Second)
	if err != nil {
		return errDaemonNotRunning
	}
	defer conn.Close()
	conn.SetDeadline(time.Now().Add(5 * time.Second))

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

func (t hookTask) Project() string {
	v, _ := t["project"].(string)
	return v
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

// readHookInput reads original and modified task JSON from stdin (taskwarrior on-modify protocol).
// On failure, rawModified contains the raw modified line (if read) so the caller can echo it
// back to stdout — taskwarrior expects the modified task on stdout even if the hook fails.
func readHookInput() (original, modified hookTask, rawModified []byte, err error) {
	scanner := bufio.NewScanner(os.Stdin)
	scanner.Buffer(make([]byte, 0, 1024*1024), 1024*1024)

	if !scanner.Scan() {
		if err := scanner.Err(); err != nil {
			return nil, nil, nil, fmt.Errorf("reading original task from stdin: %w", err)
		}
		return nil, nil, nil, fmt.Errorf("failed to read original task from stdin")
	}
	if err := json.Unmarshal(scanner.Bytes(), &original); err != nil {
		return nil, nil, nil, fmt.Errorf("failed to parse original task: %w", err)
	}

	if !scanner.Scan() {
		if err := scanner.Err(); err != nil {
			return nil, nil, nil, fmt.Errorf("reading modified task from stdin: %w", err)
		}
		return nil, nil, nil, fmt.Errorf("failed to read modified task from stdin")
	}

	rawModified = append([]byte{}, scanner.Bytes()...)

	if err := json.Unmarshal(rawModified, &modified); err != nil {
		return nil, nil, rawModified, fmt.Errorf("failed to parse modified task: %w", err)
	}

	return original, modified, rawModified, nil
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

// writeTask writes the task JSON to stdout as required by the taskwarrior
// hook protocol. The task may have been enriched in-place before this call.
func writeTask(task hookTask) {
	data, _ := json.Marshal(task)
	fmt.Println(string(data))
}

// hookLog writes a structured log line to <data_dir>/hooks.log.
func hookLog(eventType, taskUUID, description string, kvs ...string) {
	logPath := filepath.Join(config.DefaultDataDir(), "hooks.log")
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
	defer f.Close()
	f.WriteString(line)
}

// NotifyTelegram sends a notification to the team's Telegram chat using the
// dedicated notification bot token. Sends directly via Telegram API without
// going through the daemon socket.
// Fire-and-forget: errors are logged but not propagated.
func NotifyTelegram(message string) {
	if err := notify.Send(message); err != nil {
		hookLogFile(fmt.Sprintf("ERROR: telegram notify failed: %v", err))
	}
}

func hookLogFile(message string) {
	logPath := filepath.Join(config.DefaultDataDir(), "hooks.log")
	timestamp := time.Now().Format(time.RFC3339)
	f, err := os.OpenFile(logPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return
	}
	defer f.Close()
	fmt.Fprintf(f, "[%s] %s\n", timestamp, message)
}
