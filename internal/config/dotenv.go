package config

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/joho/godotenv"
)

// DotEnvPath returns the path to the .env file: ~/.config/ttal/.env
func DotEnvPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".config", "ttal", ".env"), nil
}

// LoadDotEnv reads ~/.config/ttal/.env and returns key-value pairs.
// Returns empty map (not error) if file doesn't exist.
func LoadDotEnv() (map[string]string, error) {
	path, err := DotEnvPath()
	if err != nil {
		return nil, err
	}

	env, err := godotenv.Read(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return make(map[string]string), nil
		}
		return nil, fmt.Errorf("failed to read %s: %w", path, err)
	}
	return env, nil
}
