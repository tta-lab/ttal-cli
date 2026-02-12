package worker

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

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

// hookConfig holds config for hook operations.
type hookConfig struct {
	TelegramChatID    string `json:"telegram_chat_id"`
	TelegramAccountID string `json:"telegram_account_id"`
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
	return &cfg, nil
}

// readHookInput reads original and modified task JSON from stdin (taskwarrior on-modify protocol).
func readHookInput() (original, modified hookTask, err error) {
	scanner := bufio.NewScanner(os.Stdin)
	scanner.Buffer(make([]byte, 0, 1024*1024), 1024*1024)

	if !scanner.Scan() {
		return nil, nil, fmt.Errorf("failed to read original task from stdin")
	}
	if err := json.Unmarshal(scanner.Bytes(), &original); err != nil {
		return nil, nil, fmt.Errorf("failed to parse original task: %w", err)
	}

	if !scanner.Scan() {
		return nil, nil, fmt.Errorf("failed to read modified task from stdin")
	}
	if err := json.Unmarshal(scanner.Bytes(), &modified); err != nil {
		return nil, nil, fmt.Errorf("failed to parse modified task: %w", err)
	}

	return original, modified, nil
}

// outputModifiedTask writes the modified task JSON to stdout (required by taskwarrior).
func outputModifiedTask(modified hookTask) {
	data, err := json.Marshal(modified)
	if err != nil {
		// Fallback: taskwarrior needs something on stdout
		fmt.Fprintln(os.Stderr, "ERROR: failed to marshal modified task:", err)
		return
	}
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
	defer f.Close()
	f.WriteString(line)
}

// notifyAgent sends a fire-and-forget message to the worker-lifecycle agent.
func notifyAgent(message string) {
	notifyAgentWith(message, "worker-lifecycle")
}

// notifyAgentWith sends a fire-and-forget message to a specific openclaw agent.
func notifyAgentWith(message, agent string) {
	cfg, err := loadHookConfig()
	if err != nil {
		hookLogFile("ERROR: cannot load config for agent notify: " + err.Error())
		return
	}

	if cfg.TelegramChatID == "" {
		hookLogFile("ERROR: telegram_chat_id not configured")
		return
	}

	openclaw, err := exec.LookPath("openclaw")
	if err != nil {
		hookLogFile("ERROR: openclaw command not found")
		return
	}

	accountID := cfg.TelegramAccountID
	if accountID == "" {
		accountID = "kestrel"
	}

	cmd := exec.Command(openclaw, "agent",
		"--message", message,
		"--agent", agent,
		"--deliver",
		"--channel", "telegram",
		"--reply-account", accountID,
		"--to", cfg.TelegramChatID,
	)
	cmd.Stdout = nil
	cmd.Stderr = nil

	if err := cmd.Start(); err != nil {
		hookLogFile("ERROR: failed to spawn openclaw: " + err.Error())
		return
	}

	// Fire-and-forget: release the child process
	go cmd.Wait()
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
	defer f.Close()
	fmt.Fprintf(f, "[%s] %s\n", timestamp, message)
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
