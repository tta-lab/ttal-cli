package config

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/BurntSushi/toml"
)

// LoadPrompts loads prompts from ~/.config/ttal/prompts.toml.
// Returns zero-value PromptsConfig if file doesn't exist (not an error).
func LoadPrompts() (PromptsConfig, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return PromptsConfig{}, fmt.Errorf("could not determine home directory: %w", err)
	}
	path := filepath.Join(home, ".config", "ttal", "prompts.toml")
	var prompts PromptsConfig
	if _, err := toml.DecodeFile(path, &prompts); err != nil {
		if os.IsNotExist(err) {
			return PromptsConfig{}, nil
		}
		return PromptsConfig{}, fmt.Errorf("failed to parse prompts.toml: %w", err)
	}
	return prompts, nil
}
