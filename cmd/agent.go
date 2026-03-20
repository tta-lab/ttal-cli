package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"charm.land/lipgloss/v2"
	"charm.land/lipgloss/v2/table"
	"github.com/spf13/cobra"
	"github.com/tta-lab/ttal-cli/internal/agentfs"
	"github.com/tta-lab/ttal-cli/internal/config"
	"github.com/tta-lab/ttal-cli/internal/format"
	"github.com/tta-lab/ttal-cli/internal/license"
	"github.com/tta-lab/ttal-cli/internal/voice"
)

var (
	agentVoice       string
	agentEmoji       string
	agentDescription string
	agentRole        string
)

// resolveTeamPath loads config and returns the active team's team_path.
func resolveTeamPath() (string, error) {
	cfg, err := config.Load()
	if err != nil {
		return "", fmt.Errorf("load config: %w", err)
	}
	tp := cfg.TeamPath()
	if tp == "" {
		return "", fmt.Errorf("team_path not set in config (set it in ~/.config/ttal/config.toml)")
	}
	return tp, nil
}

var agentCmd = &cobra.Command{
	Use:   "agent",
	Short: "Manage agents",
	Long:  `Add, list, get, and modify agents.`,
}

var agentAddCmd = &cobra.Command{
	Use:   "add <name>",
	Short: "Add a new agent",
	Long: `Create a new agent directory with a CLAUDE.md file.

Example:
  ttal agent add yuki --voice af_heart --emoji 🐱`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		name := strings.ToLower(args[0])

		teamPath, err := resolveTeamPath()
		if err != nil {
			return err
		}

		// Enforce agent limit based on license tier.
		lic, err := license.Load()
		if err != nil {
			return fmt.Errorf("license check: %w", err)
		}
		count, err := agentfs.Count(teamPath)
		if err != nil {
			return fmt.Errorf("count agents: %w", err)
		}
		if err := lic.CheckAgentLimit(count); err != nil {
			return err
		}

		// Check if agent already exists
		agentDir := filepath.Join(teamPath, name)
		claudeMd := filepath.Join(agentDir, "CLAUDE.md")
		if _, err := os.Stat(claudeMd); err == nil {
			return fmt.Errorf("agent '%s' already exists at %s", name, agentDir)
		}

		if err := os.MkdirAll(agentDir, 0o755); err != nil {
			return fmt.Errorf("create directory: %w", err)
		}

		// Build CLAUDE.md with optional frontmatter
		var sb strings.Builder
		hasFm := agentVoice != "" || agentEmoji != "" || agentDescription != "" || agentRole != ""
		if hasFm {
			sb.WriteString("---\n")
			if agentDescription != "" {
				fmt.Fprintf(&sb, "description: %s\n", agentDescription)
			}
			if agentEmoji != "" {
				fmt.Fprintf(&sb, "emoji: %s\n", agentEmoji)
			}
			if agentRole != "" {
				fmt.Fprintf(&sb, "role: %s\n", agentRole)
			}
			if agentVoice != "" {
				if !voice.IsValidVoice(agentVoice) {
					return fmt.Errorf("unknown voice '%s' — run 'ttal voice list' to see available voices", agentVoice)
				}
				fmt.Fprintf(&sb, "voice: %s\n", agentVoice)
			}
			sb.WriteString("---\n")
		}
		fmt.Fprintf(&sb, "# %s\n", name)

		if err := os.WriteFile(claudeMd, []byte(sb.String()), 0o644); err != nil {
			return fmt.Errorf("write CLAUDE.md: %w", err)
		}

		fmt.Printf("Agent '%s' created at %s\n", name, agentDir)
		return nil
	},
}

var agentListCmd = &cobra.Command{
	Use:   "list",
	Short: "List agents",
	RunE: func(cmd *cobra.Command, args []string) error {
		teamPath, err := resolveTeamPath()
		if err != nil {
			return err
		}

		agents, err := agentfs.Discover(teamPath)
		if err != nil {
			return fmt.Errorf("discover agents: %w", err)
		}

		if len(agents) == 0 {
			fmt.Println("No agents found")
			return nil
		}

		dimColor, headerStyle, cellStyle, dimStyle := format.TableStyles()

		rows := make([][]string, 0, len(agents))
		for _, a := range agents {
			name := a.Name
			if a.Emoji != "" {
				name = a.Emoji + " " + a.Name
			}
			rows = append(rows, []string{name, a.Role, a.Description})
		}

		tbl := table.New().
			Border(lipgloss.RoundedBorder()).
			BorderStyle(lipgloss.NewStyle().Foreground(dimColor)).
			StyleFunc(func(row, col int) lipgloss.Style {
				if row == table.HeaderRow {
					return headerStyle
				}
				if col == 0 {
					return dimStyle
				}
				return cellStyle
			}).
			Headers("NAME", "ROLE", "DESCRIPTION").
			Rows(rows...)

		fmt.Println(tbl)
		fmt.Printf("\n%d %s\n", len(agents), format.Plural(len(agents), "agent", "agents"))
		return nil
	},
}

var agentInfoCmd = &cobra.Command{
	Use:   "info <name>",
	Short: "Get agent details",
	Long: `Get detailed information about an agent.

Example:
  ttal agent info yuki`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		name := strings.ToLower(args[0])

		teamPath, err := resolveTeamPath()
		if err != nil {
			return err
		}

		ag, err := agentfs.Get(teamPath, name)
		if err != nil {
			return err
		}

		displayName := ag.Name
		if ag.Emoji != "" {
			displayName = ag.Emoji + " " + displayName
		}
		fmt.Printf("Name:      %s\n", displayName)
		fmt.Printf("Path:      %s\n", filepath.Join(teamPath, ag.Name))
		if ag.Role != "" {
			fmt.Printf("Role:      %s\n", ag.Role)
		}
		if ag.Description != "" {
			fmt.Printf("About:     %s\n", ag.Description)
		}
		if ag.Voice != "" {
			fmt.Printf("Voice:     %s\n", ag.Voice)
		}
		return nil
	},
}

var agentModifyCmd = &cobra.Command{
	Use:   "modify <name> [field:value...]",
	Short: "Modify agent metadata in CLAUDE.md frontmatter",
	Long: `Modify agent fields stored in CLAUDE.md frontmatter.

Examples:
  ttal agent modify yuki voice:af_heart
  ttal agent modify yuki emoji:🐱 description:'Task orchestration'`,
	Args: cobra.MinimumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		name := strings.ToLower(args[0])
		fieldUpdates, err := parseModifyArgs(args[1:])
		if err != nil {
			return err
		}
		if len(fieldUpdates) == 0 {
			return fmt.Errorf("no modifications specified\n\n  Example: ttal agent modify kestrel voice:af_heart emoji:🦅")
		}

		teamPath, err := resolveTeamPath()
		if err != nil {
			return err
		}

		// Verify agent exists
		if _, err := agentfs.Get(teamPath, name); err != nil {
			return err
		}

		for field, value := range fieldUpdates {
			switch field {
			case "voice":
				if !voice.IsValidVoice(value) {
					return fmt.Errorf("unknown voice '%s' — run 'ttal voice list' to see available voices", value)
				}
			case "emoji", "description", "role":
				// valid fields
			default:
				return fmt.Errorf("unknown field '%s' (available: voice, emoji, description, role)", field)
			}

			if err := agentfs.SetField(teamPath, name, field, value); err != nil {
				return fmt.Errorf("update %s: %w", field, err)
			}
		}

		fmt.Printf("Agent '%s' updated successfully\n", name)
		fmt.Println("Fields updated:")
		for field, value := range fieldUpdates {
			fmt.Printf("  %s: %s\n", field, value)
		}
		return nil
	},
}

var agentDeleteCmd = &cobra.Command{
	Use:   "delete <name>",
	Short: "Permanently delete an agent directory",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		name := strings.ToLower(args[0])

		teamPath, err := resolveTeamPath()
		if err != nil {
			return err
		}

		agentDir := filepath.Join(teamPath, name)
		if _, err := os.Stat(filepath.Join(agentDir, "CLAUDE.md")); err != nil {
			return fmt.Errorf("agent '%s' not found\n\n  List available agents: ttal agent list", name)
		}

		if !confirmPrompt(fmt.Sprintf("Permanently delete agent '%s' and its directory %s? [y/N] ", name, agentDir)) {
			fmt.Println("Aborted.")
			return nil
		}

		if err := os.RemoveAll(agentDir); err != nil {
			return fmt.Errorf("delete %s: %w", agentDir, err)
		}

		fmt.Printf("Agent '%s' deleted permanently\n", name)
		return nil
	},
}

func init() {
	rootCmd.AddCommand(agentCmd)
	agentCmd.AddCommand(agentAddCmd)
	agentCmd.AddCommand(agentListCmd)
	agentCmd.AddCommand(agentInfoCmd)
	agentCmd.AddCommand(agentModifyCmd)
	agentCmd.AddCommand(agentDeleteCmd)

	agentAddCmd.Flags().StringVar(&agentVoice, "voice", "", "Kokoro TTS voice ID (e.g. af_heart, af_sky)")
	agentAddCmd.Flags().StringVar(&agentEmoji, "emoji", "", "Display emoji (e.g. 🐱, 🦅)")
	agentAddCmd.Flags().StringVar(&agentDescription, "description", "", "Short role summary")
	agentAddCmd.Flags().StringVar(&agentRole, "role", "", "Agent role (matches [prompts] key, e.g. designer, researcher)")
}
