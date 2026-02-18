package daemon

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/BurntSushi/toml"
)

// Config is the top-level structure for ~/.config/ttal/config.toml.
//
// Zellij session is global — all agents live in the same session.
// Tab name = agent name (convention, not configurable).
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

// ConfigPath returns the default path to config.toml.
func ConfigPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".config", "ttal", "config.toml"), nil
}

// LoadConfig reads and validates ~/.config/ttal/config.toml.
func LoadConfig() (*Config, error) {
	path, err := ConfigPath()
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

	if cfg.ZellijSession == "" {
		return nil, fmt.Errorf("config missing 'zellij_session'")
	}
	if len(cfg.Agents) == 0 {
		return nil, fmt.Errorf("config has no agents defined")
	}

	return &cfg, nil
}

// WriteTemplate creates a starter config.toml with example config.
func WriteTemplate() error {
	path, err := ConfigPath()
	if err != nil {
		return err
	}

	if _, err := os.Stat(path); err == nil {
		return fmt.Errorf("config already exists: %s", path)
	}

	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}

	template := `zellij_session = "ttal-team"
chat_id = "TODO"
lifecycle_agent = "kestrel"

[agents.kestrel]
bot_token = "TODO"
`

	return os.WriteFile(path, []byte(template), 0o600)
}
