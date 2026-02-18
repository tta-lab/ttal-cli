package daemon

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

// Config is the top-level structure for ~/.ttal/daemon.json.
//
// Zellij session is global — all agents live in the same session.
// Tab name = agent name (convention, not configurable).
// ChatID is global default — agents inherit it unless they override.
type Config struct {
	ZellijSession  string                 `json:"zellij_session"`
	ChatID         string                 `json:"chat_id"`
	LifecycleAgent string                 `json:"lifecycle_agent"`
	Agents         map[string]AgentConfig `json:"agents"`
}

// AgentConfig holds per-agent Telegram credentials.
// ChatID is optional — falls back to the global Config.ChatID.
type AgentConfig struct {
	BotToken string `json:"bot_token"`
	ChatID   string `json:"chat_id,omitempty"`
}

// AgentChatID returns the effective chat ID for an agent (per-agent override or global).
func (c *Config) AgentChatID(agent string) string {
	if ac, ok := c.Agents[agent]; ok && ac.ChatID != "" {
		return ac.ChatID
	}
	return c.ChatID
}

// ConfigPath returns the default path to daemon.json.
func ConfigPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".ttal", "daemon.json"), nil
}

// LoadConfig reads and validates ~/.ttal/daemon.json.
func LoadConfig() (*Config, error) {
	path, err := ConfigPath()
	if err != nil {
		return nil, err
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("daemon config not found: %s (run: ttal daemon install)", path)
	}

	var cfg Config
	if err := json.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("invalid daemon config: %w", err)
	}

	if cfg.ZellijSession == "" {
		return nil, fmt.Errorf("daemon config missing 'zellij_session'")
	}
	if len(cfg.Agents) == 0 {
		return nil, fmt.Errorf("daemon config has no agents defined")
	}

	return &cfg, nil
}

// WriteTemplate creates a starter daemon.json with example config.
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

	template := Config{
		ZellijSession:  "ttal-team",
		ChatID:         "TODO",
		LifecycleAgent: "kestrel",
		Agents: map[string]AgentConfig{
			"kestrel": {
				BotToken: "TODO",
			},
		},
	}

	data, err := json.MarshalIndent(template, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(path, data, 0o600)
}
