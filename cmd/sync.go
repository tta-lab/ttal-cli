package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
	"github.com/tta-lab/ttal-cli/internal/config"
	"github.com/tta-lab/ttal-cli/internal/doctor"
	"github.com/tta-lab/ttal-cli/internal/sync"
)

var (
	syncDryRun bool
)

var syncCmd = &cobra.Command{
	Use:   "sync",
	Short: "Deploy subagents and skills to runtime directories",
	Long: `Reads canonical subagent .md files, skill directories, command .md files,
and RULE.md cheat sheets, then deploys them to runtime directories.

Subagents are split into runtime-specific variants:
  Claude Code → ~/.claude/agents/{name}.md
  Codex       → ~/.codex/agents/{name}.toml + ~/.codex/config.toml

Skills are deployed:
  ~/.claude/skills/{name}/ → source directory (CC)
  ~/.codex/skills/{name}/  → source directory (Codex)
  ~/.agents/skills/{name}/  → source directory (unified)

Commands are deployed as written files (variant generation):
  Claude Code → ~/.claude/skills/{name}/SKILL.md
  Codex       → ~/.codex/skills/{name}/SKILL.md
  Unified     → ~/.agents/skills/{name}/SKILL.md

Rules (RULE.md cheat sheets) are deployed as:
  Claude Code → ~/.claude/rules/{name}.md
  Codex       → inlined into ~/.codex/AGENTS.md

Configure source paths in ~/.config/ttal/config.toml:
  [sync]
  subagents_paths = ["~/clawd/docs/agents"]
  skills_paths = ["~/clawd/docs/skills"]
  commands_paths = ["~/clawd/docs/commands"]
  rules_paths = ["~/clawd/docs/skills", "~/Code/my-project"]`,
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg, err := config.Load()
		if err != nil {
			return err
		}

		syncCfg := cfg.Sync
		teamPath := cfg.TeamPath()

		hasNoPaths := len(syncCfg.SubagentsPaths) == 0 && len(syncCfg.SkillsPaths) == 0 &&
			len(syncCfg.CommandsPaths) == 0 && len(syncCfg.RulesPaths) == 0 &&
			syncCfg.GlobalPromptPath == "" && teamPath == ""
		if hasNoPaths {
			return fmt.Errorf("no sync paths configured\n\n" +
				"Add to ~/.config/ttal/config.toml:\n" +
				"  [sync]\n" +
				"  subagents_paths = [\"~/path/to/agents\"]\n" +
				"  skills_paths = [\"~/path/to/skills\"]\n" +
				"  commands_paths = [\"~/path/to/commands\"]\n" +
				"  rules_paths = [\"~/path/to/rules\"]")
		}

		configCount := 0
		agentCount := 0
		skillCount := 0
		commandCount := 0
		ruleCount := 0

		if len(syncCfg.SubagentsPaths) > 0 || cfg.TeamPath() != "" {
			if syncDryRun {
				fmt.Println("Syncing agents (dry run)...")
			} else {
				fmt.Println("Syncing agents...")
			}

			// Deploy subagents (from subagents_paths) — NOT denied
			if len(syncCfg.SubagentsPaths) > 0 {
				subResults, err := sync.DeployAgents(syncCfg.SubagentsPaths, syncDryRun)
				if err != nil {
					return fmt.Errorf("subagent sync failed: %w", err)
				}
				printAgentResults(subResults)
				agentCount += len(subResults)
			}

			// Deploy team agents (from team_path) — denied as subagents
			if teamPath != "" {
				teamResults, err := sync.DeployAgents([]string{teamPath}, syncDryRun)
				if err != nil {
					return fmt.Errorf("team agent sync failed: %w", err)
				}
				printAgentResults(teamResults)
				agentCount += len(teamResults)

				// Only deny team agents as subagents
				primaryAgentNames := make([]string, len(teamResults))
				for i, r := range teamResults {
					primaryAgentNames[i] = r.Name
				}
				denied, err := sync.DenyPrimaryAgentsAsSubagents(primaryAgentNames, syncDryRun)
				if err != nil {
					fmt.Fprintf(os.Stderr,
						"warning: agents NOT denied as subagents (settings.json update failed): %v\n", err)
				} else {
					for _, name := range denied {
						fmt.Printf("  Denied primary agent as subagent: Agent(%s)\n", name)
					}
				}
			}
		}

		// Deploy config TOMLs from team_path to ~/.config/ttal/
		if teamPath != "" {
			fmt.Println()
			if syncDryRun {
				fmt.Println("Syncing configs (dry run)...")
			} else {
				fmt.Println("Syncing configs...")
			}

			configResults, err := sync.DeployConfigs(teamPath, config.DefaultConfigDir(), syncDryRun)
			if err != nil {
				return fmt.Errorf("config sync failed: %w", err)
			}
			for _, r := range configResults {
				fmt.Printf("  %s → %s\n", shortenHome(r.Source), shortenHome(r.Dest))
			}
			configCount = len(configResults)
		}

		if len(syncCfg.SkillsPaths) > 0 {
			fmt.Println()
			if syncDryRun {
				fmt.Println("Syncing skills (dry run)...")
			} else {
				fmt.Println("Syncing skills...")
			}

			results, err := sync.DeploySkills(syncCfg.SkillsPaths, syncDryRun)
			if err != nil {
				return fmt.Errorf("skill sync failed: %w", err)
			}

			for _, r := range results {
				fmt.Printf("  %s\n", shortenHome(r.Source))
				fmt.Printf("    → %s (claude-code)\n", shortenHome(r.Dest))
				fmt.Printf("    → %s (codex)\n", shortenHome(r.CodexDest))
				fmt.Printf("    → %s (.agents)\n", shortenHome(r.AgentsSkillsDest))
			}
			skillCount = countUniqueSkills(results)
		}

		if len(syncCfg.CommandsPaths) > 0 {
			fmt.Println()
			if syncDryRun {
				fmt.Println("Syncing commands (dry run)...")
			} else {
				fmt.Println("Syncing commands...")
			}

			results, err := sync.DeployCommands(syncCfg.CommandsPaths, syncDryRun)
			if err != nil {
				return fmt.Errorf("command sync failed: %w", err)
			}

			for _, r := range results {
				fmt.Printf("  %s\n", shortenHome(r.Source))
				fmt.Printf("    → %s (claude-code)\n", shortenHome(r.CCDest))
				fmt.Printf("    → %s (codex)\n", shortenHome(r.CodexDest))
				fmt.Printf("    → %s (.agents)\n", shortenHome(r.AgentsSkillsDest))
			}
			commandCount = len(results)
		}

		if len(syncCfg.RulesPaths) > 0 {
			fmt.Println()
			if syncDryRun {
				fmt.Println("Syncing rules (dry run)...")
			} else {
				fmt.Println("Syncing rules...")
			}

			rules, err := sync.DeployRules(syncCfg.RulesPaths, syncDryRun)
			if err != nil {
				fmt.Fprintf(os.Stderr, "warning: rule deployment: %v\n", err)
			}
			for _, r := range rules {
				fmt.Printf("  %s → %s\n", shortenHome(r.Source), shortenHome(r.Dest))
			}
			ruleCount = len(rules)

			if err := sync.DeployCodexRules(rules, syncDryRun); err != nil {
				fmt.Fprintf(os.Stderr, "warning: codex rules: %v\n", err)
			}
		}

		if syncCfg.GlobalPromptPath != "" {
			fmt.Println()
			if syncDryRun {
				fmt.Println("Syncing global prompt (dry run)...")
			} else {
				fmt.Println("Syncing global prompt...")
			}

			results, err := sync.DeployGlobalPrompt(syncCfg.GlobalPromptPath, syncDryRun)
			if err != nil {
				return fmt.Errorf("global prompt sync failed: %w", err)
			}

			for _, r := range results {
				fmt.Printf("  %s\n", shortenHome(r.Source))
				fmt.Printf("    → %s (%s)\n", shortenHome(r.Dest), r.Runtime)
			}
		}

		suffix := ""
		if syncDryRun {
			suffix = " (dry run)"
		}
		fmt.Printf("\nSynced %d configs, %d agents, %d skills, %d commands, %d rules.%s\n",
			configCount, agentCount, skillCount, commandCount, ruleCount, suffix)
		return nil
	},
}

var syncSetupCmd = &cobra.Command{
	Use:   "setup",
	Short: "Generate TaskChampion sync credentials for the active team",
	Long: `Generates TaskChampion sync credentials (client_id and encryption_secret)
and writes them to {dataDir}/taskrc.sync. Ensures the team's taskrc includes the file.

Requires task_sync_url to be set in the team's config.toml section.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg, err := config.Load()
		if err != nil {
			return err
		}

		syncURL := cfg.TaskSyncURL()
		if syncURL == "" {
			return fmt.Errorf("task_sync_url not set for team %q — add it to config.toml first", cfg.TeamName())
		}

		syncFilePath := filepath.Join(cfg.DataDir(), "taskrc.sync")

		if _, err := os.Stat(syncFilePath); err == nil {
			content, err := os.ReadFile(syncFilePath)
			if err != nil {
				return fmt.Errorf("reading existing sync file: %w", err)
			}
			fmt.Printf("Sync already configured: %s\n\n%s", syncFilePath, string(content))
			return nil
		}

		if err := doctor.GenerateSyncCredentials(cfg.DataDir(), syncURL); err != nil {
			return fmt.Errorf("generating credentials: %w", err)
		}
		fmt.Printf("Generated sync credentials: %s\n", syncFilePath)

		// Ensure taskrc includes the sync file
		taskrc := cfg.TaskRC()
		content, err := os.ReadFile(taskrc)
		if err != nil {
			return fmt.Errorf("reading taskrc: %w", err)
		}
		syncInc := "include " + syncFilePath
		if !strings.Contains(string(content), syncInc) {
			f, err := os.OpenFile(taskrc, os.O_APPEND|os.O_WRONLY, 0o644)
			if err != nil {
				return fmt.Errorf("opening taskrc: %w", err)
			}
			defer f.Close()
			if _, err := f.WriteString("\n" + syncInc + "\n"); err != nil {
				return fmt.Errorf("writing taskrc include: %w", err)
			}
			fmt.Printf("Added include to %s\n", taskrc)
		}

		fmt.Println("\nSync configured. Run `task sync` to start syncing.")
		return nil
	},
}

func init() {
	rootCmd.AddCommand(syncCmd)
	syncCmd.AddCommand(syncSetupCmd)
	syncCmd.Flags().BoolVar(&syncDryRun, "dry-run", false, "Show what would be deployed without doing it")
}

// shortenHome replaces home directory prefix with ~ for display.
func shortenHome(path string) string {
	home := config.ExpandHome("~")
	abs, err := filepath.Abs(path)
	if err != nil {
		return path
	}
	if strings.HasPrefix(abs, home) {
		return "~" + abs[len(home):]
	}
	return path
}

func printAgentResults(results []sync.AgentResult) {
	for _, r := range results {
		fmt.Printf("  %s\n", shortenHome(r.Source))
		fmt.Printf("    → %s (claude-code)\n", shortenHome(r.CCDest))
		fmt.Printf("    → %s (codex)\n", shortenHome(r.CodexDest))
	}
}

func countUniqueSkills(results []sync.SkillResult) int {
	seen := make(map[string]struct{}, len(results))
	for _, r := range results {
		seen[r.Name] = struct{}{}
	}
	return len(seen)
}
