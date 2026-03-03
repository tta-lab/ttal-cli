package cmd

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/spf13/cobra"
	"github.com/tta-lab/ttal-cli/internal/config"
	"github.com/tta-lab/ttal-cli/internal/scaffold"
)

var (
	initWorkspace string
	initScaffold  string
)

var initCmd = &cobra.Command{
	Use:   "init",
	Short: "Initialize a new ttal workspace from a starter template",
	Long: `Clone the ttal-templates repo and set up a workspace with a chosen scaffold.

Run without --scaffold to see available options and pick interactively.

Examples:
  ttal init
  ttal init --scaffold basic
  ttal init --scaffold full-markdown --workspace ~/my-agents`,
	RunE: func(cmd *cobra.Command, args []string) error {
		return runInit()
	},
}

func init() {
	initCmd.Flags().StringVar(&initWorkspace, "workspace", "~/ttal-workspace",
		"Where to set up the workspace")
	initCmd.Flags().StringVar(&initScaffold, "scaffold", "",
		"Which scaffold to use (omit for interactive picker)")
	rootCmd.AddCommand(initCmd)
}

func runInit() error {
	workspace := config.ExpandHome(initWorkspace)

	// Check workspace
	if info, err := os.Stat(workspace); err == nil && info.IsDir() {
		entries, readErr := os.ReadDir(workspace)
		if readErr != nil {
			return fmt.Errorf("cannot read workspace %s: %w", workspace, readErr)
		}
		if len(entries) > 0 {
			return fmt.Errorf("workspace %s already exists and is not empty", workspace)
		}
	}

	// Clone or update templates
	fmt.Print("Fetching templates...")
	cacheDir, err := scaffold.EnsureCache()
	if err != nil {
		fmt.Println(" failed")
		return err
	}
	fmt.Println(" done")

	// List available scaffolds
	scaffolds, err := scaffold.List(cacheDir)
	if err != nil {
		return fmt.Errorf("list scaffolds: %w", err)
	}
	if len(scaffolds) == 0 {
		return fmt.Errorf("no scaffolds found in templates repo")
	}

	// If no scaffold specified, show interactive picker
	chosen := initScaffold
	if chosen == "" {
		chosen, err = pickScaffold(scaffolds)
		if err != nil {
			return err
		}
	}

	// Apply scaffold
	fmt.Printf("\nSetting up %s in %s...\n", chosen, workspace)
	if err := scaffold.Apply(cacheDir, chosen, workspace); err != nil {
		return err
	}
	fmt.Println("  Scaffold copied.")

	// Show install hint if present
	for _, s := range scaffolds {
		if s.Dir == chosen && s.InstallHint != "" {
			fmt.Printf("\n  Note: %s\n", s.InstallHint)
		}
	}

	// Install config
	if err := installInitConfig(workspace); err != nil {
		fmt.Printf("  ! Config: %v\n", err)
	}

	// Print agents found
	printInitAgents(workspace)

	// Next steps
	fmt.Println()
	fmt.Println("Next steps:")
	fmt.Printf("  1. Edit ~/.config/ttal/config.toml"+
		" — set chat_id and team_path to %s\n", workspace)
	fmt.Println("  2. Create Telegram bots via @BotFather (one per agent)")
	fmt.Println("  3. Add bot tokens to ~/.config/ttal/.env")
	fmt.Println("  4. Run: ttal sync          (deploy skills and commands)")
	fmt.Println("  5. Run: ttal doctor         (verify everything is green)")
	fmt.Println("  6. Run: ttal daemon start   (start the daemon)")

	return nil
}

// pickScaffold shows an interactive numbered picker.
func pickScaffold(scaffolds []scaffold.ScaffoldInfo) (string, error) {
	fmt.Println()
	fmt.Println("Available scaffolds:")
	fmt.Println()

	for i, s := range scaffolds {
		fmt.Printf("  %d. %s", i+1, s.Name)
		if s.Agents != "" {
			fmt.Printf(" — %s", s.Agents)
		}
		fmt.Println()
		if s.Description != "" {
			fmt.Printf("     %s\n", s.Description)
		}
		if s.InstallHint != "" {
			fmt.Printf("     %s\n", s.InstallHint)
		}
		fmt.Println()
	}

	reader := bufio.NewReader(os.Stdin)
	for {
		fmt.Printf("Pick a scaffold [1-%d]: ", len(scaffolds))
		input, err := reader.ReadString('\n')
		if err != nil {
			return "", fmt.Errorf("read input: %w", err)
		}
		input = strings.TrimSpace(input)

		if n, err := strconv.Atoi(input); err == nil && n >= 1 && n <= len(scaffolds) {
			return scaffolds[n-1].Dir, nil
		}
		for _, s := range scaffolds {
			if strings.EqualFold(input, s.Dir) || strings.EqualFold(input, s.Name) {
				return s.Dir, nil
			}
		}
		fmt.Println("  Invalid choice. Try again.")
	}
}

// installInitConfig copies the scaffold's config.toml to ~/.config/ttal/ if none exists.
func installInitConfig(workspace string) error {
	configDst, err := config.Path()
	if err != nil {
		return err
	}

	if _, err := os.Stat(configDst); err == nil {
		fmt.Printf("  Config already exists: %s\n", configDst)
		return nil
	}

	scaffoldConfig := filepath.Join(workspace, "config.toml")
	data, err := os.ReadFile(scaffoldConfig)
	if err != nil {
		return fmt.Errorf("read scaffold config: %w", err)
	}

	if err := os.MkdirAll(filepath.Dir(configDst), 0o755); err != nil {
		return err
	}
	if err := os.WriteFile(configDst, data, 0o600); err != nil {
		return err
	}
	fmt.Printf("  Installed config: %s\n", configDst)
	return nil
}

// printInitAgents scans workspace for agent directories.
func printInitAgents(workspace string) {
	entries, err := os.ReadDir(workspace)
	if err != nil {
		return
	}
	var agents []string
	for _, e := range entries {
		if !e.IsDir() || strings.HasPrefix(e.Name(), ".") {
			continue
		}
		claudeMD := filepath.Join(workspace, e.Name(), "CLAUDE.md")
		if _, err := os.Stat(claudeMD); err == nil {
			agents = append(agents, e.Name())
		}
	}
	if len(agents) > 0 {
		fmt.Printf("  Agents: %s\n", strings.Join(agents, ", "))
	}
}
