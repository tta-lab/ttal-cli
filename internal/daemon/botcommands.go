package daemon

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/tta-lab/ttal-cli/internal/config"
	"github.com/tta-lab/ttal-cli/internal/status"
	"github.com/tta-lab/ttal-cli/internal/telegram"
	"github.com/tta-lab/ttal-cli/internal/tmux"
	"gopkg.in/yaml.v3"
)

// sanitizeCommandName replaces hyphens with underscores to comply with
// Telegram's command name restriction: only [a-z0-9_] allowed.
func sanitizeCommandName(name string) string {
	return strings.ReplaceAll(name, "-", "_")
}

// BotCommand represents a Telegram bot command for the menu.
type BotCommand struct {
	Command      string `json:"command"`
	Description  string `json:"description"`
	OriginalName string `json:"-"` // original name before sanitization (for dispatch to agent)
}

// registeredCommands is the canonical list of commands the bot understands.
var registeredCommands = []BotCommand{
	{Command: "status", Description: "Show agent context usage and stats"},
	{Command: "new", Description: "Start a new conversation (reset context)"},
	{Command: "compact", Description: "Compact the current conversation context"},
	{Command: "wait", Description: "Interrupt the agent (send Escape)"},
	{Command: "restart", Description: "Restart the daemon"},
	{Command: "help", Description: "List available commands"},
}

// DiscoverCommands reads canonical command .md files from configured paths
// and returns BotCommand entries for Telegram registration.
func DiscoverCommands(commandsPaths []string) []BotCommand {
	var discovered []BotCommand
	for _, rawPath := range commandsPaths {
		dir := config.ExpandHome(rawPath)
		entries, err := os.ReadDir(dir)
		if err != nil {
			log.Printf("[commands] warning: cannot read commands_path %q: %v", rawPath, err)
			continue
		}
		for _, entry := range entries {
			if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".md") {
				continue
			}
			content, err := os.ReadFile(filepath.Join(dir, entry.Name()))
			if err != nil {
				continue
			}
			name, desc := parseCommandFrontmatter(content)
			if name == "" {
				continue
			}
			sanitized := sanitizeCommandName(name)
			if isStaticCommand(sanitized) {
				continue
			}
			discovered = append(discovered, BotCommand{
				Command:      sanitized,
				Description:  truncateDescription(desc),
				OriginalName: name,
			})
		}
	}
	return discovered
}

func parseCommandFrontmatter(content []byte) (string, string) {
	s := string(content)
	if !strings.HasPrefix(strings.TrimSpace(s), "---") {
		return "", ""
	}
	rest := s[strings.Index(s, "---")+3:]
	idx := strings.Index(rest, "\n---")
	if idx < 0 {
		return "", ""
	}
	var fm struct {
		Name        string `yaml:"name"`
		Description string `yaml:"description"`
	}
	if err := yaml.Unmarshal([]byte(rest[:idx]), &fm); err != nil {
		return "", ""
	}
	return fm.Name, fm.Description
}

// isStaticCommand checks whether name matches a built-in command.
// Callers must pass the already-sanitized name.
func isStaticCommand(name string) bool {
	for _, cmd := range registeredCommands {
		if cmd.Command == name {
			return true
		}
	}
	return false
}

// truncateDescription truncates to Telegram's 256-char limit for command descriptions.
// Uses rune-based counting for correct handling of multi-byte UTF-8 characters.
func truncateDescription(desc string) string {
	if idx := strings.Index(desc, "\n"); idx > 0 {
		desc = desc[:idx]
	}
	runes := []rune(desc)
	if len(runes) > 256 {
		desc = string(runes[:253]) + "..."
	}
	return desc
}

// AllCommands returns the full command list: static commands + discovered dynamic commands.
func AllCommands(discovered []BotCommand) []BotCommand {
	allCommands := make([]BotCommand, len(registeredCommands))
	copy(allCommands, registeredCommands)
	return append(allCommands, discovered...)
}

// RegisterBotCommands calls Telegram setMyCommands API to expose
// the command menu in the chat UI. Includes both static and discovered commands.
func RegisterBotCommands(botToken string, allCommands []BotCommand) error {

	url := fmt.Sprintf("https://api.telegram.org/bot%s/setMyCommands", botToken)

	payload := map[string]interface{}{
		"commands": allCommands,
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("marshal commands: %w", err)
	}

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Post(url, "application/json", bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("setMyCommands request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("setMyCommands returned %d: %s", resp.StatusCode, string(respBody))
	}
	return nil
}

func handleStatusCommand(teamName, _, botToken, chatID string, args []string) {
	var agents []status.AgentStatus

	if len(args) > 0 {
		// "status <agent>" → single agent
		s, err := status.ReadAgent(teamName, args[0])
		if err != nil {
			replyTelegram(botToken, chatID, "Error: "+err.Error())
			return
		}
		if s == nil {
			replyTelegram(botToken, chatID, args[0]+": no status data")
			return
		}
		agents = []status.AgentStatus{*s}
	} else {
		// "status" → all agents
		all, err := status.ReadAll(teamName)
		if err != nil {
			replyTelegram(botToken, chatID, "Error reading status: "+err.Error())
			return
		}
		agents = all
	}

	if len(agents) == 0 {
		replyTelegram(botToken, chatID, "No agent status data available")
		return
	}

	var sb strings.Builder
	for _, a := range agents {
		staleMarker := ""
		if a.IsStale(5 * time.Minute) {
			staleMarker = " (stale)"
		}
		sb.WriteString(fmt.Sprintf(
			"%s: %.0f%% ctx | %s%s\n",
			a.Agent, a.ContextUsedPct, a.ModelName, staleMarker,
		))
	}

	replyTelegram(botToken, chatID, sb.String())
}

func handleHelpCommand(botToken, chatID string, allCommands []BotCommand) {
	var sb strings.Builder
	sb.WriteString("Available commands:\n")
	for _, cmd := range allCommands {
		sb.WriteString(fmt.Sprintf("/%s — %s\n", cmd.Command, cmd.Description))
	}
	sb.WriteString("\nAnything else is sent as a message to the agent.")
	replyTelegram(botToken, chatID, sb.String())
}

func sendKeysToAgent(teamName, agentName, botToken, chatID, keys, confirmMsg string) {
	session := config.AgentSessionName(teamName, agentName)
	if err := tmux.SendKeys(session, agentName, keys); err != nil {
		replyTelegram(botToken, chatID, "Error: "+err.Error())
		return
	}
	replyTelegram(botToken, chatID, confirmMsg)
}

func sendEscToAgent(teamName, agentName, botToken, chatID string) {
	session := config.AgentSessionName(teamName, agentName)
	if err := tmux.SendRawKey(session, agentName, "Escape"); err != nil {
		replyTelegram(botToken, chatID, "Error: "+err.Error())
		return
	}
	replyTelegram(botToken, chatID, "Sent Escape — interrupting agent")
}

func replyTelegram(botToken, chatID, text string) {
	if err := telegram.SendMessage(botToken, chatID, text); err != nil {
		log.Printf("[telegram] reply failed: %v", err)
	}
}
