package cmd

import (
	"fmt"
	"os"
	"sort"
	"time"

	"github.com/spf13/cobra"
	"github.com/tta-lab/ttal-cli/internal/agentfs"
	"github.com/tta-lab/ttal-cli/internal/config"
	"github.com/tta-lab/ttal-cli/internal/daemon"
	"github.com/tta-lab/ttal-cli/internal/runtime"
	"github.com/tta-lab/ttal-cli/internal/status"
	"github.com/tta-lab/ttal-cli/internal/tmux"
)

const staleThreshold = 5 * time.Minute

var statusCmd = &cobra.Command{
	Use:   "status [team-name]",
	Short: "Show agent status and context usage",
	Long: `Shows all agents in the active team with session health and context usage.

Without team name: uses TTAL_TEAM env or default_team from config.
With team name: shows that team's status.

Examples:
  ttal status              # active team
  ttal status guion        # specific team`,
	Args: cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		if len(args) == 1 {
			_ = os.Setenv("TTAL_TEAM", args[0])
		}
		return showStatus()
	},
}

type agentRow struct {
	name    string
	runtime string
	health  string // ✓, ✗, ~, ●
	active  bool
	ctxPct  float64 // -1 if no data
	model   string
	updated string
}

func showStatus() error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}

	running, _, _ := daemon.IsRunning()
	if !running {
		fmt.Printf("Team: %s (daemon not running)\n", cfg.TeamName())
		return nil
	}

	teamName := cfg.TeamName()
	names, _ := agentfs.DiscoverAgents(cfg.TeamPath())

	rows := make([]agentRow, 0, len(names))
	for _, name := range names {
		rows = append(rows, buildAgentRow(cfg, teamName, name))
	}

	sortAgentRows(rows)

	fmt.Printf("Team: %s\n", teamName)
	fmt.Printf("  %-12s %-14s %-6s %-10s %s\n",
		"AGENT", "RUNTIME", "CTX", "MODEL", "UPDATED")

	active := 0
	for _, r := range rows {
		ctx := "---"
		if r.ctxPct >= 0 {
			ctx = fmt.Sprintf("%.0f%%", r.ctxPct)
		}
		model := r.model
		if model == "" {
			model = "---"
		}

		fmt.Printf("  %s %-12s %-14s %-6s %-10s %s\n",
			r.health, r.name, r.runtime, ctx, model, r.updated)

		if r.active {
			active++
		}
	}

	fmt.Printf("\n%d agents | %d active\n", len(rows), active)
	return nil
}

func buildAgentRow(cfg *config.Config, teamName, name string) agentRow {
	rt := cfg.AgentRuntimeFor(name)
	sessionName := config.AgentSessionName(teamName, name)
	s, _ := status.ReadAgent(teamName, name)

	row := agentRow{
		name:    name,
		runtime: string(rt),
		ctxPct:  -1,
	}

	switch rt {
	case runtime.ClaudeCode:
		populateCCRow(&row, sessionName, s)
	default:
		row.health = "?"
		row.updated = "unknown runtime"
	}

	return row
}

func populateCCRow(row *agentRow, sessionName string, s *status.AgentStatus) {
	sessionUp := tmux.SessionExists(sessionName)
	if s != nil && !s.IsStale(staleThreshold) {
		row.health = "✓"
		row.active = true
		row.ctxPct = s.ContextUsedPct
		row.model = shortModel(s.ModelName)
		age := time.Since(s.UpdatedAt).Truncate(time.Second)
		row.updated = fmt.Sprintf("%s ago", age)
	} else if sessionUp {
		row.health = "✓"
		row.active = true
		row.updated = "no data"
	} else {
		row.health = "✗"
		row.updated = "stopped"
	}
}

// sortAgentRows sorts by context usage descending, agents without data at bottom, stable by name.
func sortAgentRows(rows []agentRow) {
	sort.SliceStable(rows, func(i, j int) bool {
		if rows[i].ctxPct >= 0 && rows[j].ctxPct < 0 {
			return true
		}
		if rows[i].ctxPct < 0 && rows[j].ctxPct >= 0 {
			return false
		}
		if rows[i].ctxPct >= 0 && rows[j].ctxPct >= 0 {
			if rows[i].ctxPct != rows[j].ctxPct {
				return rows[i].ctxPct > rows[j].ctxPct
			}
			return rows[i].name < rows[j].name
		}
		if rows[i].active != rows[j].active {
			return rows[i].active
		}
		return rows[i].name < rows[j].name
	})
}

// shortModel returns a short display name for the model.
func shortModel(name string) string {
	if len(name) > 8 {
		return name[:8]
	}
	return name
}

func init() {
	rootCmd.AddCommand(statusCmd)
}
