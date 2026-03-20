package worker

import (
	"fmt"
	"path/filepath"
	"sort"

	"charm.land/lipgloss/v2"
	"charm.land/lipgloss/v2/table"
	"github.com/tta-lab/ttal-cli/internal/enrichment"
	"github.com/tta-lab/ttal-cli/internal/format"
	"github.com/tta-lab/ttal-cli/internal/project"
	"github.com/tta-lab/ttal-cli/internal/taskwarrior"
)

// WorkerStatus represents the categorized status of a worker.
type WorkerStatus int

const (
	StatusRunning WorkerStatus = iota // No PR yet
	StatusWithPR                      // PR created
)

func (s WorkerStatus) String() string {
	switch s {
	case StatusRunning:
		return "RUNNING"
	case StatusWithPR:
		return "WITH_PR"
	default:
		return "UNKNOWN"
	}
}

// WorkerInfo holds a worker task with its derived status.
type WorkerInfo struct {
	Task   taskwarrior.Task
	Status WorkerStatus
}

// List queries active worker tasks and prints a table view.
func List() error {
	tasks, err := taskwarrior.GetActiveWorkerTasks()
	if err != nil {
		return fmt.Errorf("failed to query workers: %w", err)
	}

	if len(tasks) == 0 {
		fmt.Println("No active workers")
		fmt.Println("\nTo spawn a worker:")
		fmt.Println("  ttal task go <uuid>")
		return nil
	}

	// Deduplicate by session ID (keep first, which is most recent from taskwarrior)
	seen := make(map[string]bool)
	unique := make([]taskwarrior.Task, 0, len(tasks))
	for _, t := range tasks {
		sid := t.SessionID()
		if sid == "" || seen[sid] {
			continue
		}
		seen[sid] = true
		unique = append(unique, t)
	}

	// Categorize and build worker info
	workers := categorizeWorkers(unique)

	// Sort: group by status (RUNNING → WITH_PR),
	// within each group sort by start time descending (most recent first).
	sort.Slice(workers, func(i, j int) bool {
		if workers[i].Status != workers[j].Status {
			return workers[i].Status < workers[j].Status
		}
		// Taskwarrior timestamps sort lexicographically (YYYYMMDDTHHmmssZ)
		return workers[i].Task.Start > workers[j].Task.Start
	})

	// Print table
	printWorkerTable(workers)
	return nil
}

func categorizeWorkers(tasks []taskwarrior.Task) []WorkerInfo {
	workers := make([]WorkerInfo, 0, len(tasks))
	for _, t := range tasks {
		info := WorkerInfo{Task: t}
		if t.PRID == "" {
			info.Status = StatusRunning
		} else {
			info.Status = StatusWithPR
		}
		workers = append(workers, info)
	}
	return workers
}

func formatPRCell(prid string) string {
	if prid == "" {
		return "-"
	}
	info, err := taskwarrior.ParsePRID(prid)
	if err != nil {
		return "#" + prid
	}
	if info.LGTM {
		return fmt.Sprintf("#%d ✓", info.Index)
	}
	return fmt.Sprintf("#%d", info.Index)
}

func printWorkerTable(workers []WorkerInfo) {
	// Count by status
	counts := map[WorkerStatus]int{}
	for _, w := range workers {
		counts[w.Status]++
	}

	fmt.Printf("Active Workers: %d", len(workers))
	var parts []string
	if n := counts[StatusRunning]; n > 0 {
		parts = append(parts, fmt.Sprintf("%d running", n))
	}
	if n := counts[StatusWithPR]; n > 0 {
		parts = append(parts, fmt.Sprintf("%d with PR", n))
	}
	if len(parts) > 0 {
		fmt.Printf(" (")
		for i, p := range parts {
			if i > 0 {
				fmt.Printf(", ")
			}
			fmt.Printf("%s", p)
		}
		fmt.Printf(")")
	}
	fmt.Println()
	fmt.Println()

	dimColor, headerStyle, cellStyle, dimStyle := format.TableStyles()

	rows := make([][]string, 0, len(workers))
	for _, info := range workers {
		t := info.Task

		pr := formatPRCell(t.PRID)

		branch, _ := WorktreeBranch(t.UUID, t.Project)
		if branch == "" {
			branch = enrichment.GenerateBranch(t.Description)
		}
		if branch == "" {
			branch = "-"
		}

		projectDisplay := "-"
		if resolvedPath := project.ResolveProjectPath(t.Project); resolvedPath != "" {
			projectDisplay = filepath.Base(resolvedPath)
		}

		desc := t.Description
		if len(desc) > 60 {
			desc = desc[:57] + "..."
		}

		rows = append(rows, []string{t.SessionName(), info.Status.String(), pr, branch, projectDisplay, desc})
	}

	tbl := table.New().
		Border(lipgloss.RoundedBorder()).
		BorderStyle(lipgloss.NewStyle().Foreground(dimColor)).
		StyleFunc(func(row, col int) lipgloss.Style {
			if row == table.HeaderRow {
				return headerStyle
			}
			switch col {
			case 0, 1, 2:
				return dimStyle
			default:
				return cellStyle
			}
		}).
		Headers("SESSION", "STATUS", "PR", "BRANCH", "PROJECT", "TASK").
		Rows(rows...)

	fmt.Println(tbl)
}
