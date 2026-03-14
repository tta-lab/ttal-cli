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
	syncClean  bool
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

Commands are deployed as written files (variant generation):
  Claude Code → ~/.claude/skills/{name}/SKILL.md
  Codex       → ~/.codex/skills/{name}/SKILL.md

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

		hasNoPaths := len(syncCfg.SubagentsPaths) == 0 && len(syncCfg.SkillsPaths) == 0 &&
			len(syncCfg.CommandsPaths) == 0 && len(syncCfg.RulesPaths) == 0 && syncCfg.GlobalPromptPath == ""
		if hasNoPaths {
			return fmt.Errorf("no sync paths configured\n\n" +
				"Add to ~/.config/ttal/config.toml:\n" +
				"  [sync]\n" +
				"  subagents_paths = [\"~/path/to/agents\"]\n" +
				"  skills_paths = [\"~/path/to/skills\"]\n" +
				"  commands_paths = [\"~/path/to/commands\"]\n" +
				"  rules_paths = [\"~/path/to/rules\"]")
		}

		agentCount := 0
		skillCount := 0
		commandCount := 0
		ruleCount := 0

		// Collect agent paths: subagents_paths + team_path (if exists)
		agentPaths := make([]string, len(syncCfg.SubagentsPaths))
		copy(agentPaths, syncCfg.SubagentsPaths)
		if teamPath := cfg.TeamPath(); teamPath != "" {
			agentPaths = append(agentPaths, teamPath)
		}

		if len(agentPaths) > 0 {
			if syncDryRun {
				fmt.Println("Syncing agents (dry run)...")
			} else {
				fmt.Println("Syncing agents...")
			}

			results, err := sync.DeployAgents(agentPaths, syncDryRun)
			if err != nil {
				return fmt.Errorf("agent sync failed: %w", err)
			}

			for _, r := range results {
				fmt.Printf("  %s\n", shortenHome(r.Source))
				fmt.Printf("    → %s (claude-code)\n", shortenHome(r.CCDest))
				fmt.Printf("    → %s (opencode)\n", shortenHome(r.OCDest))
				fmt.Printf("    → %s (codex)\n", shortenHome(r.CodexDest))
			}
			agentCount = len(results)

			if syncClean {
				removed, err := sync.CleanAgents(agentPaths, syncDryRun)
				if err != nil {
					return fmt.Errorf("agent cleanup failed: %w", err)
				}
				for _, path := range removed {
					fmt.Printf("  Removed stale: %s\n", shortenHome(path))
				}
			}
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
			}
			skillCount = countUniqueSkills(results)

			if syncClean {
				removed, err := sync.CleanSkills(syncCfg.SkillsPaths, syncDryRun)
				if err != nil {
					return fmt.Errorf("skill cleanup failed: %w", err)
				}
				for _, path := range removed {
					fmt.Printf("  Removed stale: %s\n", shortenHome(path))
				}
			}
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
				fmt.Printf("    → %s (opencode)\n", shortenHome(r.OCDest))
				fmt.Printf("    → %s (codex)\n", shortenHome(r.CodexDest))
			}
			commandCount = len(results)

			if syncClean {
				removed, err := sync.CleanCommands(syncCfg.CommandsPaths, syncDryRun)
				if err != nil {
					return fmt.Errorf("command cleanup failed: %w", err)
				}
				for _, path := range removed {
					fmt.Printf("  Removed stale: %s\n", shortenHome(path))
				}
			}
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

			if syncClean {
				removed, err := sync.CleanRules(syncCfg.RulesPaths, syncDryRun)
				if err != nil {
					fmt.Fprintf(os.Stderr, "warning: rule cleanup: %v\n", err)
				}
				for _, path := range removed {
					fmt.Printf("  Removed stale: %s\n", shortenHome(path))
				}
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
		fmt.Printf("\nSynced %d agents, %d skills, %d commands, %d rules.%s\n",
			agentCount, skillCount, commandCount, ruleCount, suffix)
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
	syncCmd.Flags().BoolVar(&syncClean, "clean", false, "Remove deployed agents/skills that no longer exist in source")
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

func countUniqueSkills(results []sync.SkillResult) int {
	seen := make(map[string]struct{}, len(results))
	for _, r := range results {
		seen[r.Name] = struct{}{}
	}
	return len(seen)
}
