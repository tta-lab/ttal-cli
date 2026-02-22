package daemon

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strings"
	"time"

	"codeberg.org/clawteam/ttal-cli/internal/config"
	"codeberg.org/clawteam/ttal-cli/internal/status"
	"codeberg.org/clawteam/ttal-cli/internal/telegram"
	"codeberg.org/clawteam/ttal-cli/internal/tmux"
)

// BotCommand represents a Telegram bot command for the menu.
type BotCommand struct {
	Command     string `json:"command"`
	Description string `json:"description"`
}

// registeredCommands is the canonical list of commands the bot understands.
var registeredCommands = []BotCommand{
	{Command: "status", Description: "Show agent context usage and stats"},
	{Command: "new", Description: "Start a new conversation (reset context)"},
	{Command: "compact", Description: "Compact the current conversation context"},
	{Command: "wait", Description: "Interrupt the agent (send Escape)"},
	{Command: "help", Description: "List available commands"},
}

// RegisterBotCommands calls Telegram setMyCommands API to expose
// the command menu in the chat UI.
func RegisterBotCommands(botToken string) error {
	url := fmt.Sprintf("https://api.telegram.org/bot%s/setMyCommands", botToken)

	payload := map[string]interface{}{
		"commands": registeredCommands,
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
	defer resp.Body.Close() //nolint:errcheck

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("setMyCommands returned %d", resp.StatusCode)
	}
	return nil
}

func handleStatusCommand(_, botToken, chatID string, args []string) {
	var agents []status.AgentStatus

	if len(args) > 0 {
		// "status <agent>" → single agent
		s, err := status.ReadAgent(args[0])
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
		all, err := status.ReadAll()
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

func handleHelpCommand(botToken, chatID string) {
	var sb strings.Builder
	sb.WriteString("Available commands:\n")
	for _, cmd := range registeredCommands {
		sb.WriteString(fmt.Sprintf("/%s — %s\n", cmd.Command, cmd.Description))
	}
	sb.WriteString("\nAnything else is sent as a message to the agent.")
	replyTelegram(botToken, chatID, sb.String())
}

func sendKeysToAgent(agentName, botToken, chatID, keys, confirmMsg string) {
	session := config.AgentSessionName(agentName)
	if err := tmux.SendKeys(session, agentName, keys); err != nil {
		replyTelegram(botToken, chatID, "Error: "+err.Error())
		return
	}
	replyTelegram(botToken, chatID, confirmMsg)
}

func sendEscToAgent(agentName, botToken, chatID string) {
	session := config.AgentSessionName(agentName)
	// SendKeys with literal "Escape" — but we need raw tmux send-keys without -l
	// Use tmux.SendRawKey for special keys
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
