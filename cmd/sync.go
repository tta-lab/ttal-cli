package cmd

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
	"github.com/tta-lab/ttal-cli/internal/config"
	"github.com/tta-lab/ttal-cli/internal/doctor"
	"github.com/tta-lab/ttal-cli/internal/project"
	"github.com/tta-lab/ttal-cli/internal/sync"
)

var (
	syncDryRun bool
)

var syncCmd = &cobra.Command{
	Use:   "sync",
	Short: "Deploy plugin, rules, and configs to runtime directories",
	Long: `Installs the ttal CC plugin (subagents + SessionStart hook) and deploys
RULE.md cheat sheets and config TOMLs.

Plugin (subagents + hook):
  Installed via CC plugin marketplace (claude plugin install ttal@ttal)

Rules (RULE.md cheat sheets) are deployed as:
  Claude Code → ~/.claude/rules/{name}.md
  Codex       → inlined into ~/.codex/AGENTS.md

Config TOMLs are deployed from team_path:
  prompts.toml, roles.toml, pipelines.toml → ~/.config/ttal/
  config.toml is NOT synced (machine-specific settings).

Configure source paths in ~/.config/ttal/config.toml:
  [sync]
  rules_paths = ["~/clawd/skills", "~/Code/my-project"]`,
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg, err := config.Load()
		if err != nil {
			return err
		}

		syncCfg := cfg.Sync
		teamPath := cfg.TeamPath()

		// Plugin install always runs (resolves marketplace from project store or URL).
		// Only error if there's nothing else to sync either.
		hasNoPaths := len(syncCfg.RulesPaths) == 0 &&
			syncCfg.GlobalPromptPath == "" && teamPath == "" &&
			syncCfg.MarketplaceSource == "" && project.ResolveProjectPath("ttal") == ""
		if hasNoPaths {
			return fmt.Errorf("no sync paths configured\n\n" +
				"Add to ~/.config/ttal/config.toml:\n" +
				"  [sync]\n" +
				"  rules_paths = [\"~/path/to/rules\"]")
		}

		configCount := 0
		ruleCount := 0

		// Install/update ttal CC plugin (subagents + SessionStart hook).
		printSyncHeader("plugin", syncDryRun)
		marketplaceSrc := syncCfg.MarketplaceSource
		if marketplaceSrc == "" {
			// Resolve from project store — local clone preferred.
			marketplaceSrc = project.ResolveProjectPath("ttal")
		}
		if marketplaceSrc == "" {
			marketplaceSrc = "https://github.com/tta-lab/ttal-cli"
		}
		pluginResult, err := sync.InstallPlugin(marketplaceSrc, syncDryRun)
		if err != nil {
			fmt.Fprintf(os.Stderr, "warning: plugin sync failed: %v\n", err)
		} else {
			if pluginResult.MarketplaceAdded {
				fmt.Printf("  Marketplace registered: %s\n", shortenHome(marketplaceSrc))
			}
			if pluginResult.PluginInstalled {
				fmt.Printf("  Plugin installed: ttal (%d agents, SessionStart hook)\n", pluginResult.AgentCount)
			} else if pluginResult.PluginUpdated {
				fmt.Printf("  Plugin updated: ttal (%d agents, SessionStart hook)\n", pluginResult.AgentCount)
			} else {
				fmt.Printf("  Plugin: ttal (up to date, %d agents)\n", pluginResult.AgentCount)
			}
		}

		// Deploy config TOMLs from team_path to ~/.config/ttal/
		if teamPath != "" {
			printSyncHeader("configs", syncDryRun)

			configResults, err := sync.DeployConfigs(teamPath, config.DefaultConfigDir(), syncDryRun)
			if err != nil {
				return fmt.Errorf("config sync failed: %w", err)
			}
			for _, r := range configResults {
				fmt.Printf("  %s → %s\n", shortenHome(r.Source), shortenHome(r.Dest))
			}
			configCount = len(configResults)
		}

		if len(syncCfg.RulesPaths) > 0 {
			printSyncHeader("rules", syncDryRun)

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
			printSyncHeader("global prompt", syncDryRun)

			results, err := sync.DeployGlobalPrompt(syncCfg.GlobalPromptPath, syncDryRun)
			if err != nil {
				return fmt.Errorf("global prompt sync failed: %w", err)
			}

			for _, r := range results {
				fmt.Printf("  %s\n", shortenHome(r.Source))
				fmt.Printf("    → %s (%s)\n", shortenHome(r.Dest), r.Runtime)
			}
		}

		// Sync sandbox via einai (ei) if available in PATH
		printSyncHeader("sandbox", syncDryRun)
		if _, err := exec.LookPath("ei"); err == nil {
			if !syncDryRun {
				if err := exec.Command("ei", "sandbox", "sync").Run(); err != nil {
					fmt.Fprintf(os.Stderr, "  warning: ei sandbox sync failed: %v\n", err)
				} else {
					fmt.Printf("  ei sandbox synced\n")
				}
			} else {
				fmt.Printf("  ei sandbox sync (dry run)\n")
			}
		} else {
			fmt.Printf("  skipped: ei not in PATH (run 'go install github.com/tta-lab/einai/cmd/ei@latest' to enable sandbox sync)\n")
		}

		suffix := ""
		if syncDryRun {
			suffix = " (dry run)"
		}
		fmt.Printf("\nSynced %d configs, %d rules.%s\n",
			configCount, ruleCount, suffix)
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

// printSyncHeader prints a blank line followed by a section header for a sync phase.
func printSyncHeader(label string, dryRun bool) {
	fmt.Println()
	if dryRun {
		fmt.Printf("Syncing %s (dry run)...\n", label)
	} else {
		fmt.Printf("Syncing %s...\n", label)
	}
}
