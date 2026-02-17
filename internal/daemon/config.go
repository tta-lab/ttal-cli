package daemon

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

// Config is the top-level structure for ~/.ttal/daemon.json.
type Config struct {
	Agents map[string]AgentConfig `json:"agents"`
}

// AgentConfig holds per-agent delivery settings.
type AgentConfig struct {
	Telegram TelegramConfig `json:"telegram"`
	Zellij   ZellijConfig   `json:"zellij"`
}

// TelegramConfig holds bot credentials for one agent.
type TelegramConfig struct {
	BotToken string `json:"bot_token"`
	ChatID   string `json:"chat_id"`
}

// ZellijConfig holds zellij delivery target for one agent.
type ZellijConfig struct {
	Session string `json:"session"`
	Tab     string `json:"tab"`
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
		Agents: map[string]AgentConfig{
			"kestrel": {
				Telegram: TelegramConfig{
					BotToken: "123:ABC...",
					ChatID:   "845849177",
				},
				Zellij: ZellijConfig{
					Session: "cclaw",
					Tab:     "kestrel",
				},
			},
		},
	}

	data, err := json.MarshalIndent(template, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(path, data, 0o600)
}
