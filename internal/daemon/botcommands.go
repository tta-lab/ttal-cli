package daemon

import (
	"encoding/json"
	"log"
	"os/exec"
	"strings"
)

// skillEntry mirrors the JSON schema emitted by `skill list --json`.
type skillEntry struct {
	Name        string `json:"name"`
	Category    string `json:"category"`
	Source      string `json:"source"`
	Description string `json:"description"`
}

// sanitizeCommandName replaces hyphens with underscores to comply with
// Telegram's command name restriction: only [a-z0-9_] allowed.
func sanitizeCommandName(name string) string {
	return strings.ReplaceAll(name, "-", "_")
}

// BotCommand represents a bot command for the menu.
type BotCommand struct {
	Command      string `json:"command"`
	Description  string `json:"description"`
	OriginalName string `json:"-"` // original name before sanitization (for dispatch to agent)
}

// registeredCommands is the canonical list of static commands the bot understands.
var registeredCommands = []BotCommand{
	{Command: "status", Description: "Show agent context usage and stats"},
	{Command: "usage", Description: "Show Claude API usage (5-hour and weekly limits)"},
	{Command: "new", Description: "Start a new conversation (reset context)"},
	{Command: "compact", Description: "Compact the current conversation context"},
	{Command: "wait", Description: "Interrupt the agent (send Escape)"},
	{Command: "restart", Description: "Restart the daemon"},
	{Command: "help", Description: "List available commands"},
	{Command: "save", Description: "Save agent's last message to FlickNote"},
}

// DiscoverCommands runs `skill list --json` and extracts command-category skills.
func DiscoverCommands() []BotCommand {
	skillPath, err := exec.LookPath("skill")
	if err != nil {
		log.Printf("[commands] skill binary not found in PATH — bot commands disabled: %v", err)
		return nil
	}

	out, err := exec.Command(skillPath, "list", "--json").Output()
	if err != nil {
		log.Printf("[commands] ERROR: cannot run 'skill list --json' — dynamic commands unavailable: %v", err)
		return nil
	}

	var skills []skillEntry
	if err := json.Unmarshal(out, &skills); err != nil {
		log.Printf("[commands] ERROR: cannot parse 'skill list --json' output: %v", err)
		return nil
	}

	return discoverCommandsFromSkills(skills)
}

// discoverCommandsFromSkills extracts command-category skills as BotCommands.
func discoverCommandsFromSkills(skills []skillEntry) []BotCommand {
	var discovered []BotCommand
	for _, s := range skills {
		if s.Category != "command" {
			continue
		}
		sanitized := sanitizeCommandName(s.Name)
		if isStaticBotCommand(sanitized) {
			continue
		}
		discovered = append(discovered, BotCommand{
			Command:      sanitized,
			Description:  truncateDescription(s.Description),
			OriginalName: s.Name,
		})
	}
	return discovered
}

// isStaticBotCommand checks whether name matches a built-in command.
// Callers must pass the already-sanitized name.
func isStaticBotCommand(name string) bool {
	for _, cmd := range registeredCommands {
		if cmd.Command == name {
			return true
		}
	}
	return false
}

// truncateDescription truncates to Telegram's 256-char limit for command descriptions.
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
	allCommands := make([]BotCommand, 0, len(registeredCommands)+len(discovered))
	allCommands = append(allCommands, registeredCommands...)
	return append(allCommands, discovered...)
}
