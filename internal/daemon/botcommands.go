package daemon

import (
	"log"
	"os"
	"path/filepath"
	"strings"

	"github.com/tta-lab/ttal-cli/internal/config"
	"gopkg.in/yaml.v3"
)

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
}

// DiscoverCommands reads canonical command .md files from configured paths
// and returns BotCommand entries for registration.
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
			if isStaticBotCommand(sanitized) {
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
	allCommands := make([]BotCommand, len(registeredCommands))
	copy(allCommands, registeredCommands)
	return append(allCommands, discovered...)
}
