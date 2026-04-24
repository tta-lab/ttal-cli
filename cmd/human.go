package cmd

import (
	"fmt"
	"os"

	"charm.land/lipgloss/v2"
	"charm.land/lipgloss/v2/table"
	"github.com/spf13/cobra"
	"github.com/tta-lab/ttal-cli/internal/config"
	"github.com/tta-lab/ttal-cli/internal/humanfs"
)

var humanCmd = &cobra.Command{
	Use:   "human",
	Short: "Manage humans",
	Long:  `List and get information about humans in the team.`,
}

var humanListCmd = &cobra.Command{
	Use:   "list",
	Short: "List all humans",
	Long:  `List all humans defined in humans.toml.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		humansPath, err := config.HumansPath()
		if err != nil {
			return fmt.Errorf("humans path: %w", err)
		}
		humans, err := humanfs.Load(humansPath)
		if err != nil {
			if os.IsNotExist(err) {
				return fmt.Errorf("no humans.toml found — run ttal doctor --fix to generate one")
			}
			return fmt.Errorf("load humans: %w", err)
		}

		if len(humans) == 0 {
			fmt.Println("No humans configured.")
			return nil
		}

		t := table.New().
			Border(lipgloss.RoundedBorder()).
			StyleFunc(func(row, col int) lipgloss.Style {
				switch {
				case row == 0:
					return lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("99"))
				case col == 3 && humans[row-1].Admin:
					return lipgloss.NewStyle().Foreground(lipgloss.Color("10"))
				default:
					return lipgloss.NewStyle().Foreground(lipgloss.Color("252"))
				}
			}).
			Headers("ALIAS", "NAME", "AGE", "ADMIN", "TELEGRAM", "MATRIX")

		for _, h := range humans {
			admin := ""
			if h.Admin {
				admin = "yes"
			}
			t.Row(h.Alias, h.Name, fmt.Sprintf("%d", h.Age), admin,
				truncate(h.TelegramChatID, 12), truncate(h.MatrixUserID, 20))
		}

		fmt.Println(t.Render())
		return nil
	},
}

var humanInfoCmd = &cobra.Command{
	Use:   "info <alias>",
	Short: "Show details for a human",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		humansPath, err := config.HumansPath()
		if err != nil {
			return fmt.Errorf("humans path: %w", err)
		}
		h, err := humanfs.Get(humansPath, args[0])
		if err != nil {
			return fmt.Errorf("human %q not found", args[0])
		}

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
	},
}

func truncate(s string, max int) string {
	if len(s) > max {
		return s[:max] + "…"
	}
	return s
}

func init() {
	humanCmd.AddCommand(humanListCmd)
	humanCmd.AddCommand(humanInfoCmd)
	rootCmd.AddCommand(humanCmd)
}
