package cmd

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"

	"charm.land/lipgloss/v2"
	"charm.land/lipgloss/v2/table"
	"github.com/spf13/cobra"
	"github.com/tta-lab/ttal-cli/internal/agentfs"
	"github.com/tta-lab/ttal-cli/internal/config"
	"github.com/tta-lab/ttal-cli/internal/format"
	"github.com/tta-lab/ttal-cli/internal/humanfs"
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
	tp := cfg.TeamPath
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
	Long: `Create a new agent directory with an AGENTS.md file.

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
		claudeMd := filepath.Join(agentDir, "AGENTS.md")
		if _, err := os.Stat(claudeMd); err == nil {
			return fmt.Errorf("agent '%s' already exists at %s", name, agentDir)
		}

		if err := os.MkdirAll(agentDir, 0o755); err != nil {
			return fmt.Errorf("create directory: %w", err)
		}

		// Build AGENTS.md with optional frontmatter
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
			return fmt.Errorf("write AGENTS.md: %w", err)
		}

		fmt.Printf("Agent '%s' created at %s\n", name, agentDir)
		return nil
	},
}

var agentListCmd = &cobra.Command{
	Use:   "list",
	Short: "List agents (humans + AIs)",
	RunE: func(cmd *cobra.Command, args []string) error {
		teamPath, err := resolveTeamPath()
		if err != nil {
			return err
		}

		agents, err := agentfs.Discover(teamPath)
		if err != nil {
			return fmt.Errorf("discover agents: %w", err)
		}

		// Load humans (best-effort — empty if humans.toml absent).
		var humans []humanfs.Human
		if humansPath, herr := config.HumansPath(); herr == nil {
			if loaded, lerr := humanfs.Load(humansPath); lerr == nil {
				humans = loaded
			} else if !os.IsNotExist(lerr) {
				log.Printf("[agent list] warning: humans.toml unreadable: %v", lerr)
			}
		}

		if len(agents) == 0 && len(humans) == 0 {
			fmt.Println("No agents or humans found")
			return nil
		}

		dimColor, headerStyle, cellStyle, dimStyle := format.TableStyles()

		// Build rows: humans first (alphabetical), then AIs (alphabetical from Discover).
		rows := make([][]string, 0, len(humans)+len(agents))
		for _, h := range humans {
			rows = append(rows, []string{h.Alias, "human", "", h.Name})
		}
		for _, a := range agents {
			name := a.Name
			if a.Emoji != "" {
				name = a.Emoji + " " + a.Name
			}
			rows = append(rows, []string{name, "ai", a.Role, a.Description})
		}

		tbl := table.New().
			Border(lipgloss.RoundedBorder()).
			BorderStyle(lipgloss.NewStyle().Foreground(dimColor)).
			StyleFunc(func(row, col int) lipgloss.Style {
				if row == table.HeaderRow {
					return headerStyle
				}
				if row < 0 || row >= len(rows) {
					return cellStyle
				}
				if col == 0 {
					return dimStyle
				}
				return cellStyle
			}).
			Headers("NAME", "TYPE", "ROLE", "DESCRIPTION").
			Rows(rows...)

		lipgloss.Println(tbl)
		fmt.Printf("\n%d %s (%d %s, %d %s)\n",
			len(rows), format.Plural(len(rows), "addressee", "addressees"),
			len(humans), format.Plural(len(humans), "human", "humans"),
			len(agents), format.Plural(len(agents), "ai", "ais"))
		return nil
	},
}

var agentInfoCmd = &cobra.Command{
	Use:   "info <name>",
	Short: "Get addressee details (human or ai)",
	Long: `Get detailed information about an agent or human.

Example:
  ttal agent info yuki
  ttal agent info neil`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		name := strings.ToLower(args[0])

		// Try human first (matches resolveAddressee order).
		if humansPath, err := config.HumansPath(); err == nil {
			if h, herr := humanfs.Get(humansPath, name); herr == nil {
				return printHumanInfo(h)
			}
		}

		// Fall back to AI agent.
		teamPath, err := resolveTeamPath()
		if err != nil {
			return err
		}
		ag, err := agentfs.Get(teamPath, name)
		if err != nil {
			return err
		}
		return printAgentInfo(teamPath, ag)
	},
}

var agentModifyCmd = &cobra.Command{
	Use:   "modify <name> [field:value...]",
	Short: "Modify agent metadata in AGENTS.md frontmatter",
	Long: `Modify agent fields stored in AGENTS.md frontmatter.

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
			case "emoji", "description", "role", "color":
				// valid fields — color accepts any string; no validation (unlike voice)
			default:
				return fmt.Errorf("unknown field '%s' (available: voice, emoji, description, role, color)", field)
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

func printAgentInfo(teamPath string, ag *agentfs.AgentInfo) error {
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
	if ag.Color != "" {
		fmt.Printf("Color:     %s\n", ag.Color)
	}
	return nil
}

func printHumanInfo(h *humanfs.Human) error {
	label := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("99"))
	value := lipgloss.NewStyle().Foreground(lipgloss.Color("252"))
	fmt.Printf("%s  %s\n", label.Render("Alias:"), value.Render(h.Alias))
	fmt.Printf("%s  %s\n", label.Render("Name:"), value.Render(h.Name))
	fmt.Printf("%s  %s\n", label.Render("Age:"), value.Render(fmt.Sprintf("%d", h.Age)))
	fmt.Printf("%s  %s\n", label.Render("Pronouns:"), value.Render(h.Pronouns))
	fmt.Printf("%s  %s\n", label.Render("Admin:"), value.Render(fmt.Sprintf("%t", h.Admin)))
	fmt.Printf("%s  %s\n", label.Render("Telegram:"), value.Render(h.TelegramChatID))
	fmt.Printf("%s  %s\n", label.Render("Matrix:"), value.Render(h.MatrixUserID))
	return nil
}

func truncate(s string, max int) string {
	if len(s) > max {
		return s[:max] + "…"
	}
	return s
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
		if _, err := os.Stat(filepath.Join(agentDir, "AGENTS.md")); err != nil {
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
