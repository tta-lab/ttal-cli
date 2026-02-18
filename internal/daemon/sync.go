package daemon

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
)

// SyncTokens reads {AGENT}_BOT_TOKEN env vars for each agent in daemon.json
// and writes the tokens into the config file.
func SyncTokens() error {
	path, err := ConfigPath()
	if err != nil {
		return err
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("daemon config not found: %s (run: ttal daemon install)", path)
	}

	// Use raw map to preserve unknown fields and ordering
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		return fmt.Errorf("invalid daemon config: %w", err)
	}

	var agents map[string]AgentConfig
	if err := json.Unmarshal(raw["agents"], &agents); err != nil {
		return fmt.Errorf("invalid agents config: %w", err)
	}

	updated := 0
	skipped := 0

	for name, ac := range agents {
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
		agents[name] = ac
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

	agentsJSON, err := json.Marshal(agents)
	if err != nil {
		return err
	}
	raw["agents"] = agentsJSON

	out, err := json.MarshalIndent(raw, "", "  ")
	if err != nil {
		return err
	}

	if err := os.WriteFile(path, out, 0o600); err != nil {
		return err
	}

	fmt.Printf("\nUpdated %d token(s) in %s\n", updated, path)
	return nil
}
