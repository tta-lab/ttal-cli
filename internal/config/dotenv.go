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

// InjectDotEnvFallback loads ~/.config/ttal/.env and sets any key that is not
// already present in the environment. Returns an error if the file cannot be
// read (callers typically print a warning and continue).
func InjectDotEnvFallback() error {
	dotEnv, err := LoadDotEnv()
	if err != nil {
		return err
	}
	for k, v := range dotEnv {
		if os.Getenv(k) == "" {
			_ = os.Setenv(k, v)
		}
	}
	return nil
}

// DotEnvParts loads .env and returns "KEY=VALUE" strings suitable for
// appending to an environment variable slice. All errors (missing file,
// parse failures, unreadable path) are silently ignored — returns nil.
func DotEnvParts() []string {
	env, err := LoadDotEnv()
	if err != nil {
		return nil
	}
	parts := make([]string, 0, len(env))
	for k, v := range env {
		parts = append(parts, fmt.Sprintf("%s=%s", k, v))
	}
	return parts
}
