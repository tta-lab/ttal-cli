package worker

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"text/tabwriter"

	"github.com/guion-opensource/ttal-cli/internal/forgejo"
	"github.com/guion-opensource/ttal-cli/internal/taskwarrior"
)

// WorkerStatus represents the categorized status of a worker.
type WorkerStatus int

const (
	StatusRunning       WorkerStatus = iota // No PR yet
	StatusWithPR                            // PR created, not merged
	StatusCleanupNeeded                     // PR merged, needs cleanup
)

func (s WorkerStatus) String() string {
	switch s {
	case StatusRunning:
		return "RUNNING"
	case StatusWithPR:
		return "WITH_PR"
	case StatusCleanupNeeded:
		return "CLEANUP"
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
		fmt.Println("  ttal worker spawn --name <name> --project <dir> --task <uuid>")
		return nil
	}

	// Deduplicate by session_name (keep first, which is most recent from taskwarrior)
	seen := make(map[string]bool)
	unique := make([]taskwarrior.Task, 0, len(tasks))
	for _, t := range tasks {
		if t.SessionName == "" || seen[t.SessionName] {
			continue
		}
		seen[t.SessionName] = true
		unique = append(unique, t)
	}

	// Categorize and build worker info
	workers := categorizeWorkers(unique)

	// Sort: group by status (RUNNING → WITH_PR → CLEANUP),
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
			if checkPRMerged(t) {
				info.Status = StatusCleanupNeeded
			} else {
				info.Status = StatusWithPR
			}
		}

		workers = append(workers, info)
	}
	return workers
}

func checkPRMerged(t taskwarrior.Task) bool {
	if t.ProjectPath == "" {
		return false
	}

	owner, repo, err := forgejo.ParseRepoInfo(t.ProjectPath)
	if err != nil {
		return false
	}

	prID, err := strconv.ParseInt(t.PRID, 10, 64)
	if err != nil {
		return false
	}

	merged, err := forgejo.IsPRMerged(owner, repo, prID)
	if err != nil {
		return false
	}
	return merged
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
	if n := counts[StatusCleanupNeeded]; n > 0 {
		parts = append(parts, fmt.Sprintf("%d need cleanup", n))
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

	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	_, _ = fmt.Fprintln(w, "SESSION\tSTATUS\tPR\tBRANCH\tPROJECT\tTASK")

	for _, info := range workers {
		t := info.Task

		pr := "-"
		if t.PRID != "" {
			pr = "#" + t.PRID
		}

		branch := t.Branch
		if branch == "" {
			branch = "-"
		}

		project := "-"
		if t.ProjectPath != "" {
			project = filepath.Base(t.ProjectPath)
		}

		desc := t.Description
		if len(desc) > 60 {
			desc = desc[:57] + "..."
		}

		_, _ = fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\t%s\n",
			t.SessionName, info.Status, pr, branch, project, desc)
	}

	_ = w.Flush()
}
