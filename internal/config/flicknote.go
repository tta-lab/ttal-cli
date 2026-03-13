package config

import (
	"os"
	"path/filepath"
)

// FlicknoteHooksDir returns the path to the flicknote hooks directory.
// Currently ~/.config/flicknote/hooks/ — when XDG_CONFIG_HOME support
// is added to ttal, update this single function.
func FlicknoteHooksDir() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".config", "flicknote", "hooks"), nil
}
