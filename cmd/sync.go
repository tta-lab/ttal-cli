package cmd

import (
	"fmt"
	"path/filepath"
	"strings"

	"codeberg.org/clawteam/ttal-cli/internal/config"
	"codeberg.org/clawteam/ttal-cli/internal/sync"
	"github.com/spf13/cobra"
)

var (
	syncDryRun bool
	syncClean  bool
)

var syncCmd = &cobra.Command{
	Use:   "sync",
	Short: "Deploy subagents and skills to runtime directories",
	Long: `Reads canonical subagent .md files, skill directories, and command definitions,
then deploys them to Claude Code and OpenCode runtime directories.

Subagents are split into runtime-specific variants:
  Claude Code → ~/.claude/agents/{name}.md
  OpenCode    → ~/.config/opencode/agents/{name}.md

Skills are symlinked:
  ~/.claude/skills/{name}/ → source directory

Commands are deployed as runtime-specific variants:
  Claude Code → ~/.claude/skills/{name}/SKILL.md (CC treats commands as skills)
  OpenCode    → ~/.config/opencode/commands/{name}.md

Configure source paths in ~/.config/ttal/config.toml:
  [sync]
  subagents_paths = ["~/clawd/docs/agents"]
  skills_paths = ["~/clawd/docs/skills"]
  commands_paths = ["~/clawd/docs/commands"]`,
	// No database needed
	PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
		return nil
	},
	PersistentPostRunE: func(cmd *cobra.Command, args []string) error {
		return nil
	},
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg, err := config.Load()
		if err != nil {
			return err
		}

		syncCfg := cfg.Sync

		if len(syncCfg.SubagentsPaths) == 0 && len(syncCfg.SkillsPaths) == 0 && len(syncCfg.CommandsPaths) == 0 {
			return fmt.Errorf("no sync paths configured\n\n" +
				"Add to ~/.config/ttal/config.toml:\n" +
				"  [sync]\n" +
				"  subagents_paths = [\"~/path/to/agents\"]\n" +
				"  skills_paths = [\"~/path/to/skills\"]\n" +
				"  commands_paths = [\"~/path/to/commands\"]")
		}

		agentCount := 0
		skillCount := 0
		commandCount := 0

		if len(syncCfg.SubagentsPaths) > 0 {
			if syncDryRun {
				fmt.Println("Syncing subagents (dry run)...")
			} else {
				fmt.Println("Syncing subagents...")
			}

			results, err := sync.DeployAgents(syncCfg.SubagentsPaths, syncDryRun)
			if err != nil {
				return fmt.Errorf("agent sync failed: %w", err)
			}

			for _, r := range results {
				fmt.Printf("  %s\n", shortenHome(r.Source))
				fmt.Printf("    → %s (claude-code)\n", shortenHome(r.CCDest))
				fmt.Printf("    → %s (opencode)\n", shortenHome(r.OCDest))
			}
			agentCount = len(results)

			if syncClean {
				removed, err := sync.CleanAgents(syncCfg.SubagentsPaths, syncDryRun)
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
				fmt.Printf("  %s → %s (symlink)\n", shortenHome(r.Source), shortenHome(r.Dest))
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

			cmdResults, cmdErr := sync.DeployCommands(syncCfg.CommandsPaths, syncDryRun)
			if cmdErr != nil {
				return fmt.Errorf("command sync failed: %w", cmdErr)
			}

			for _, r := range cmdResults {
				fmt.Printf("  %s\n", shortenHome(r.Source))
				fmt.Printf("    → %s (claude-code)\n", shortenHome(r.CCDest))
				fmt.Printf("    → %s (opencode)\n", shortenHome(r.OCDest))
			}
			commandCount = len(cmdResults)

			if syncClean {
				removed, cleanErr := sync.CleanCommands(syncCfg.CommandsPaths, syncDryRun)
				if cleanErr != nil {
					return fmt.Errorf("command cleanup failed: %w", cleanErr)
				}
				for _, path := range removed {
					fmt.Printf("  Removed stale: %s\n", shortenHome(path))
				}
			}
		}

		suffix := ""
		if syncDryRun {
			suffix = " (dry run)"
		}
		fmt.Printf("\nSynced %d subagents, %d skills, %d commands.%s\n", agentCount, skillCount, commandCount, suffix)
		return nil
	},
}

func init() {
	rootCmd.AddCommand(syncCmd)
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
