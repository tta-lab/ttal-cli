package config

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/BurntSushi/toml"
)

// AgentSessionName returns the zellij session name for an agent.
// Convention: "session-<agent-name>". Derived, not stored.
func AgentSessionName(agent string) string {
	return "session-" + agent
}

// Config is the top-level structure for ~/.config/ttal/config.toml.
//
// ZellijSession is deprecated — sessions are now per-agent (session-<name>).
// Kept for backward compatibility but no longer required.
// ChatID is global default — agents inherit it unless they override.
type Config struct {
	ZellijSession  string                 `toml:"zellij_session"`
	ChatID         string                 `toml:"chat_id"`
	LifecycleAgent string                 `toml:"lifecycle_agent"`
	Agents         map[string]AgentConfig `toml:"agents"`
}

// AgentConfig holds per-agent Telegram credentials.
// ChatID is optional — falls back to the global Config.ChatID.
type AgentConfig struct {
	BotToken string `toml:"bot_token"`
	ChatID   string `toml:"chat_id"`
}

// AgentChatID returns the effective chat ID for an agent (per-agent override or global).
func (c *Config) AgentChatID(agent string) string {
	if ac, ok := c.Agents[agent]; ok && ac.ChatID != "" {
		return ac.ChatID
	}
	return c.ChatID
}

// Path returns the default path to config.toml.
func Path() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".config", "ttal", "config.toml"), nil
}

// Load reads and validates ~/.config/ttal/config.toml.
func Load() (*Config, error) {
	path, err := Path()
	if err != nil {
		return nil, err
	}

	var cfg Config
	if _, err := toml.DecodeFile(path, &cfg); err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("config not found: %s (run: ttal daemon install)", path)
		}
		return nil, fmt.Errorf("invalid config: %w", err)
	}

	if len(cfg.Agents) == 0 {
		return nil, fmt.Errorf("config has no agents defined")
	}

	return &cfg, nil
}

// WriteTemplate creates a starter config.toml with example config.
func WriteTemplate() error {
	path, err := Path()
	if err != nil {
		return err
	}

	if _, err := os.Stat(path); err == nil {
		return fmt.Errorf("config already exists: %s", path)
	}

	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}

	template := `chat_id = "TODO"
lifecycle_agent = "kestrel"

[agents.kestrel]
bot_token = "TODO"
`

	return os.WriteFile(path, []byte(template), 0o600)
}
