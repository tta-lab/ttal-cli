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
			tryAutoAdvanceStage0(task, p)
		}
	}

	writeTask(task)
}

// tryAutoAdvanceStage0 auto-advances past stage 0 when the creating agent's role matches
// the first stage assignee, preventing double-routing back to the same agent.
func tryAutoAdvanceStage0(task hookTask, p *pipeline.Pipeline) {
	if p == nil || len(p.Stages) == 0 {
		return
	}
	agentName := os.Getenv("TTAL_AGENT_NAME")
	if agentName == "" {
		return
	}
	teamPath := resolveTeamPathForHook()
	if teamPath == "" {
		return
	}
	agent, err := agentfs.Get(teamPath, agentName)
	if err != nil {
		hookLogFile("WARN: could not resolve agent for stage-0 skip: " + err.Error())
		return
	}
	if agent.Role != p.Stages[0].Assignee {
		return
	}
	task.SetTag(agentName)
	task.SetStart()
	hookLog("PIPELINE-SKIP", task.UUID(), task.Description(),
		"agent", agentName, "role", agent.Role, "stage", p.Stages[0].Name)
}

// resolveTeamPathForHook resolves the team path from config for use in hooks.
// Returns "" if config can't be loaded or team path is not set.
func resolveTeamPathForHook() string {
	cfg, err := config.Load()
	if err != nil {
		hookLogFile("WARN: could not load config for stage-0 skip: " + err.Error())
		return ""
	}
	return cfg.TeamPath()
}
