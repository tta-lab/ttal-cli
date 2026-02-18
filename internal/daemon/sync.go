package daemon

import (
	"fmt"
	"os"
	"strings"

	"github.com/BurntSushi/toml"
)

// SyncTokens reads {AGENT}_BOT_TOKEN env vars for each agent in config.toml
// and writes the tokens into the config file.
func SyncTokens() error {
	cfg, err := LoadConfig()
	if err != nil {
		return err
	}

	path, err := ConfigPath()
	if err != nil {
		return err
	}

	updated := 0
	skipped := 0

	for name, ac := range cfg.Agents {
		envVar := strings.ToUpper(name) + "_BOT_TOKEN"
		token := os.Getenv(envVar)
		if token == "" {
			fmt.Printf("  %-12s  skip  (%s not set)\n", name, envVar)
			skipped++
			continue
		}

		if ac.BotToken == token {
			fmt.Printf("  %-12s  ok    (unchanged)\n", name)
			continue
		}

		ac.BotToken = token
		cfg.Agents[name] = ac
		updated++
		fmt.Printf("  %-12s  set   (from %s)\n", name, envVar)
	}

	if updated == 0 {
		fmt.Printf("\nNo changes to write.\n")
		if skipped > 0 {
			fmt.Printf("Set env vars and retry: export <AGENT>_BOT_TOKEN=\"...\"\n")
		}
		return nil
	}

	f, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0o600)
	if err != nil {
		return err
	}
	defer f.Close() //nolint:errcheck

	if err := toml.NewEncoder(f).Encode(cfg); err != nil {
		return err
	}

	fmt.Printf("\nUpdated %d token(s) in %s\n", updated, path)
	return nil
}
