package worker

import (
	"fmt"
	"os"

	"github.com/tta-lab/ttal-cli/internal/agentfs"
	"github.com/tta-lab/ttal-cli/internal/config"
	"github.com/tta-lab/ttal-cli/internal/pipeline"
)

// HookOnAdd handles the taskwarrior on-add event.
// Reads one JSON line from stdin, enriches inline if project is set, outputs to stdout.
func HookOnAdd() {
	task, rawLine, err := readHookAddInput()
	if err != nil {
		hookLogFile("ERROR in on-add: " + err.Error())
		if len(rawLine) > 0 {
			fmt.Println(string(rawLine))
		}
		os.Exit(0)
	}

	hookLog("ADD", task.UUID(), task.Description())

	// Inline enrichment — validates project alias.
	if task.Project() != "" {
		if err := enrichInline(task, nil); err != nil {
			hookLogFile("ERROR: " + err.Error())
			fmt.Fprintln(os.Stderr, err.Error())
			os.Exit(1)
		}
	}

	// Validate pipeline tag overlaps and auto-advance past stage 0 when applicable.
	if len(task.Tags()) > 0 {
		configDir := config.DefaultConfigDir()
		pipelineCfg, err := pipeline.Load(configDir)
		if err != nil {
			// Malformed pipelines.toml — warn but don't block task creation.
			fmt.Fprintf(os.Stderr, "warning: pipelines.toml: %v\n", err)
		} else {
			_, p, matchErr := pipelineCfg.MatchPipeline(task.Tags())
			if matchErr != nil {
				hookLogFile("ERROR pipeline conflict: " + matchErr.Error())
				fmt.Fprintln(os.Stderr, "pipeline conflict: "+matchErr.Error())
				os.Exit(1)
			}

			// Auto-advance past stage 0 when creating agent's role matches the first stage assignee.
			// Prevents double-routing: if a designer creates a +feature task, they're already on it.
			if p != nil && len(p.Stages) > 0 {
				agentName := os.Getenv("TTAL_AGENT_NAME")
				if agentName != "" {
					teamPath := resolveTeamPathForHook()
					if teamPath != "" {
						if agent, err := agentfs.Get(teamPath, agentName); err == nil {
							if agent.Role == p.Stages[0].Assignee {
								task.SetTag(agentName)
								task.SetStart()
								hookLog("PIPELINE-SKIP", task.UUID(), task.Description(),
									"agent", agentName, "role", agent.Role, "stage", p.Stages[0].Name)
							}
						}
					}
				}
			}
		}
	}

	writeTask(task)
}

// resolveTeamPathForHook resolves the team path from config for use in hooks.
// Returns "" if config can't be loaded or team path is not set.
func resolveTeamPathForHook() string {
	cfg, err := config.Load()
	if err != nil {
		return ""
	}
	return cfg.TeamPath()
}
