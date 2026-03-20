package worker

import (
	"fmt"
	"os"

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

	// Validate pipeline tag overlaps — block task creation if ambiguous.
	if len(task.Tags()) > 0 {
		configDir := config.DefaultConfigDir()
		pipelineCfg, err := pipeline.Load(configDir)
		if err != nil {
			// Malformed pipelines.toml — warn but don't block task creation.
			fmt.Fprintf(os.Stderr, "warning: pipelines.toml: %v\n", err)
		} else {
			if _, _, err := pipelineCfg.MatchPipeline(task.Tags()); err != nil {
				hookLogFile("ERROR pipeline conflict: " + err.Error())
				fmt.Fprintln(os.Stderr, "pipeline conflict: "+err.Error())
				os.Exit(1)
			}
		}
	}

	writeTask(task)
}
