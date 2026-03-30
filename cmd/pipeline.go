package cmd

import (
	"fmt"
	"strings"

	"charm.land/lipgloss/v2"
	"charm.land/lipgloss/v2/table"
	"github.com/spf13/cobra"
	"github.com/tta-lab/ttal-cli/internal/config"
	"github.com/tta-lab/ttal-cli/internal/format"
	"github.com/tta-lab/ttal-cli/internal/pipeline"
)

var pipelineCmd = &cobra.Command{
	Use:   "pipeline",
	Short: "Inspect pipeline definitions",
	Long:  `List and inspect pipeline definitions from pipelines.toml.`,
}

var pipelineListCmd = &cobra.Command{
	Use:   "list",
	Short: "List all pipelines",
	Long: `List all pipeline definitions with their descriptions and matching tags.

Example:
  ttal pipeline list`,
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg, err := pipeline.Load(config.DefaultConfigDir())
		if err != nil {
			return fmt.Errorf("load pipelines: %w", err)
		}
		if len(cfg.Pipelines) == 0 {
			fmt.Println("No pipelines configured")
			return nil
		}

		dimColor, headerStyle, cellStyle, _ := format.TableStyles()

		names := cfg.SortedNames()
		rows := make([][]string, 0, len(names))
		for _, name := range names {
			p := cfg.Pipelines[name]
			rows = append(rows, []string{
				name,
				p.Description,
				strings.Join(p.Tags, ", "),
			})
		}

		t := table.New().
			Border(lipgloss.RoundedBorder()).
			BorderStyle(lipgloss.NewStyle().Foreground(dimColor)).
			StyleFunc(func(row, col int) lipgloss.Style {
				if row == table.HeaderRow {
					return headerStyle
				}
				return cellStyle
			}).
			Headers("NAME", "DESCRIPTION", "TAGS").
			Rows(rows...)

		lipgloss.Println(t)
		fmt.Printf("\n%d %s\n", len(names), format.Plural(len(names), "pipeline", "pipelines"))
		return nil
	},
}

var pipelineGetCmd = &cobra.Command{
	Use:   "get <name>",
	Short: "Show pipeline stages",
	Long: `Show the stages of a pipeline as a simple terminal graph.

Example:
  ttal pipeline get standard
  ttal pipeline get bugfix`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg, err := pipeline.Load(config.DefaultConfigDir())
		if err != nil {
			return fmt.Errorf("load pipelines: %w", err)
		}

		name := args[0]
		p, ok := cfg.Pipelines[name]
		if !ok {
			return fmt.Errorf("pipeline %q not found\n\nAvailable: %s", name, strings.Join(cfg.SortedNames(), ", "))
		}

		fmt.Printf("%s — %s\n", name, p.Description)
		fmt.Printf("Tags: %s\n\n", strings.Join(p.Tags, ", "))
		renderPipelineGraph(p)
		return nil
	},
}

// renderPipelineGraph prints pipeline stages as a linear graph.
// Example output:
//
//	Plan [designer] ──human──▸ Implement [coder]
//
// Reviewer info is shown inline when present:
//
//	Plan [designer] ──human/plan-review-lead──▸ Implement [coder]
//
// Skills are shown in parentheses when present:
//
//	Plan [designer] (sp-planning, flicknote) ──human/plan-review-lead──▸ Implement [coder]
func renderPipelineGraph(p pipeline.Pipeline) {
	for i, s := range p.Stages {
		if i > 0 {
			// Print arrow from previous stage to this one.
			prev := p.Stages[i-1]
			label := prev.Gate
			if prev.Reviewer != "" {
				label += "/" + prev.Reviewer
			}
			fmt.Printf(" ──%s──▸ ", label)
		}
		fmt.Printf("%s [%s]", s.Name, s.Assignee)
		if len(s.Skills) > 0 {
			fmt.Printf(" (%s)", strings.Join(s.Skills, ", "))
		}
	}
	fmt.Println()
}

func init() {
	rootCmd.AddCommand(pipelineCmd)
	pipelineCmd.AddCommand(pipelineListCmd)
	pipelineCmd.AddCommand(pipelineGetCmd)
}
