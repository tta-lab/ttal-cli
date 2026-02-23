package config

import (
	"fmt"
	"os"
	"strings"

	"github.com/BurntSushi/toml"
)

// SyncTokens reads {AGENT}_BOT_TOKEN env vars for the given agents and writes
// the tokens into config.toml. Agent names come from the ttal database (SSOT).
// New agents are added to the config automatically if they have a token set.
func SyncTokens(agentNames []string) error {
	cfg, err := Load()
	if err != nil {
		return err
	}

	path, err := Path()
	if err != nil {
		return err
	}

	if cfg.Agents == nil {
		cfg.Agents = make(map[string]AgentConfig)
	}

	updated := 0
	skipped := 0

	for _, name := range agentNames {
		envVar := strings.ToUpper(name) + "_BOT_TOKEN"
		token := os.Getenv(envVar)
		if token == "" {
			fmt.Printf("  %-12s  skip  (%s not set)\n", name, envVar)
			skipped++
			continue
		}

		ac := cfg.Agents[name]
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

	// For team configs, sync the agents map back into the active team
	// and clear flat fields to avoid duplication.
	if len(cfg.Teams) > 0 {
		if team, ok := cfg.Teams[cfg.resolvedTeamName]; ok {
			team.Agents = cfg.Agents
			cfg.Teams[cfg.resolvedTeamName] = team
		}
		cfg.clearResolvedFields()
	}

	f, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0o600)
	if err != nil {
		return err
	}
	defer f.Close()

	if err := toml.NewEncoder(f).Encode(cfg); err != nil {
		return err
	}

	fmt.Printf("\nUpdated %d token(s) in %s\n", updated, path)
	return nil
}
