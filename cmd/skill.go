package cmd

import (
	"encoding/json"
	"fmt"
	"strings"

	"charm.land/lipgloss/v2"
	"charm.land/lipgloss/v2/table"
	"github.com/spf13/cobra"
	"github.com/tta-lab/ttal-cli/internal/format"
	"github.com/tta-lab/ttal-cli/internal/skill"
)

var skillCmd = &cobra.Command{
	Use:   "skill",
	Short: "Manage skills",
	Long:  `List, get, and find skills deployed to ~/.agents/skills/.`,
}

var skillListCmd = &cobra.Command{
	Use:   "list",
	Short: "List available skills",
	Long: `List skills deployed to ~/.agents/skills/.

Example:
  ttal skill list
  ttal skill list --all`,
	RunE: func(cmd *cobra.Command, args []string) error {
		listAll, _ := cmd.Flags().GetBool("all")
		jsonOut, _ := cmd.Flags().GetBool("json")
		return runSkillList(listAll, jsonOut)
	},
}

var skillGetCmd = &cobra.Command{
	Use:   "get <name>",
	Short: "Print skill content",
	Long: `Print the content of a skill by name.

Example:
  ttal skill get breathe
  ttal skill get sp-debugging`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		jsonOut, _ := cmd.Flags().GetBool("json")
		return runSkillGet(args[0], jsonOut)
	},
}

var skillFindCmd = &cobra.Command{
	Use:   "find <keyword> [keyword...]",
	Short: "Search skills by keyword",
	Long: `Search skills by keyword (OR match) against name, description, and content.

Example:
  ttal skill find debug
  ttal skill find git commit`,
	Args: cobra.MinimumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		return runSkillFind(args)
	},
}

func init() {
	rootCmd.AddCommand(skillCmd)
	skillCmd.AddCommand(skillListCmd)
	skillCmd.AddCommand(skillGetCmd)
	skillCmd.AddCommand(skillFindCmd)

	skillListCmd.Flags().Bool("all", false, "List all skills")
	skillListCmd.Flags().Bool("json", false, "Output as JSON")
	skillGetCmd.Flags().Bool("json", false, "Output as JSON")
	skillFindCmd.Flags().Bool("all", false, "Search all skills")
}

// buildSkillTable returns a lipgloss table with skill styling.
// dimCols lists column indices that should receive dim styling.
func buildSkillTable(headers []string, rows [][]string, dimCols ...int) *table.Table {
	dimColSet := make(map[int]bool, len(dimCols))
	for _, c := range dimCols {
		dimColSet[c] = true
	}
	dimColor, headerStyle, cellStyle, dimStyle := format.TableStyles()
	return table.New().
		Border(lipgloss.RoundedBorder()).
		BorderStyle(lipgloss.NewStyle().Foreground(dimColor)).
		StyleFunc(func(row, col int) lipgloss.Style {
			if row == table.HeaderRow {
				return headerStyle
			}
			if dimColSet[col] {
				return dimStyle
			}
			return cellStyle
		}).
		Headers(headers...).
		Rows(rows...)
}

type skillJSON struct {
	Name        string `json:"name"`
	Category    string `json:"category"`
	Description string `json:"description"`
}

type skillJSONWithContent struct {
	Name        string `json:"name"`
	Category    string `json:"category"`
	Description string `json:"description"`
	Content     string `json:"content"`
}

func runSkillList(listAll, jsonOut bool) error {
	skills, err := skill.ListSkills(skill.DefaultSkillsDir())
	if err != nil {
		return fmt.Errorf("listing skills: %w", err)
	}

	if len(skills) == 0 {
		fmt.Println("No skills found.")
		return nil
	}

	if jsonOut {
		output := make([]skillJSON, 0, len(skills))
		for _, s := range skills {
			output = append(output, skillJSON{
				Name:        s.Name,
				Category:    s.Category,
				Description: s.Description,
			})
		}
		return printJSON(output)
	}

	showCategory := listAll

	var rows [][]string
	if showCategory {
		for _, s := range skills {
			rows = append(rows, []string{s.Name, s.Category, s.Description})
		}
		lipgloss.Println(buildSkillTable([]string{"Name", "Category", "Description"}, rows, 1))
	} else {
		for _, s := range skills {
			rows = append(rows, []string{s.Name, s.Description})
		}
		lipgloss.Println(buildSkillTable([]string{"Name", "Description"}, rows))
	}
	return nil
}

func runSkillGet(name string, jsonOut bool) error {
	s, err := skill.GetSkill(skill.DefaultSkillsDir(), name)
	if err != nil {
		return fmt.Errorf("skill %q not found", name)
	}

	if jsonOut {
		return printJSON(skillJSONWithContent{
			Name:        s.Name,
			Category:    s.Category,
			Description: s.Description,
			Content:     s.Content,
		})
	}

	fmt.Print(s.Content)
	return nil
}

func printJSON(v any) error {
	data, err := json.Marshal(v)
	if err != nil {
		return fmt.Errorf("failed to marshal skill: %w", err)
	}
	fmt.Println(string(data))
	return nil
}

type matchSource string

const (
	matchName matchSource = "name"
)

type findResultDisk struct {
	Name        string
	Description string
	Category    string
	source      matchSource
}

// findByNameDesc returns skills whose name or description matches any keyword.
func findByNameDesc(candidates []skill.DiskSkill, keywords []string) map[string]*findResultDisk {
	results := make(map[string]*findResultDisk)
	for _, s := range candidates {
		for _, kw := range keywords {
			kwLower := strings.ToLower(kw)
			if strings.Contains(strings.ToLower(s.Name), kwLower) ||
				strings.Contains(strings.ToLower(s.Description), kwLower) {
				sc := s
				results[s.Name] = &findResultDisk{Name: sc.Name, Description: sc.Description,
					Category: sc.Category, source: matchName}
				break
			}
		}
	}
	return results
}

func runSkillFind(keywords []string) error {
	skills, err := skill.ListSkills(skill.DefaultSkillsDir())
	if err != nil {
		return fmt.Errorf("listing skills: %w", err)
	}

	results := findByNameDesc(skills, keywords)

	if len(results) == 0 {
		fmt.Println("No skills found.")
		return nil
	}

	sorted := make([]*findResultDisk, 0, len(results))
	for _, r := range results {
		sorted = append(sorted, r)
	}
	for i := 1; i < len(sorted); i++ {
		for j := i; j > 0 && sorted[j].Name < sorted[j-1].Name; j-- {
			sorted[j], sorted[j-1] = sorted[j-1], sorted[j]
		}
	}

	var rows [][]string
	for _, res := range sorted {
		rows = append(rows, []string{res.Name, res.Category, string(res.source), res.Description})
	}

	lipgloss.Println(buildSkillTable([]string{"Name", "Category", "Match", "Description"}, rows, 1, 2))
	return nil
}
