package gitprovider

import (
	"context"
	"fmt"
	"os"
	"strings"

	woodpecker "go.woodpecker-ci.org/woodpecker/v3/woodpecker-go/woodpecker"
	"golang.org/x/oauth2"
)

// IsWoodpeckerContext returns true if a commit status context
// was created by Woodpecker CI.
func IsWoodpeckerContext(ctx string) bool {
	return strings.HasPrefix(ctx, "ci/woodpecker")
}

// WoodpeckerClient wraps the Woodpecker SDK for CI failure details.
type WoodpeckerClient struct {
	client woodpecker.Client
}

// NewWoodpeckerClient creates a Woodpecker API client from env vars.
// Requires WOODPECKER_URL and WOODPECKER_TOKEN.
func NewWoodpeckerClient() (*WoodpeckerClient, error) {
	url := os.Getenv("WOODPECKER_URL")
	if url == "" {
		return nil, fmt.Errorf("WOODPECKER_URL environment variable is required")
	}
	token := os.Getenv("WOODPECKER_TOKEN")
	if token == "" {
		return nil, fmt.Errorf("WOODPECKER_TOKEN environment variable is required")
	}

	ts := oauth2.StaticTokenSource(&oauth2.Token{AccessToken: token})
	httpClient := oauth2.NewClient(context.Background(), ts)
	client := woodpecker.NewClient(url, httpClient)

	return &WoodpeckerClient{client: client}, nil
}

// GetFailureDetails fetches failed pipeline steps and their logs for a commit SHA.
func (w *WoodpeckerClient) GetFailureDetails(owner, repo, sha string) ([]*JobFailure, error) {
	fullName := owner + "/" + repo
	repoObj, err := w.client.RepoLookup(fullName)
	if err != nil {
		return nil, fmt.Errorf("repo lookup %s: %w", fullName, err)
	}

	// List recent pipelines and find ones matching this SHA.
	// No server-side SHA filter — filter client-side.
	pipelines, err := w.client.PipelineList(repoObj.ID, woodpecker.PipelineListOptions{
		ListOptions: woodpecker.ListOptions{PerPage: 25},
	})
	if err != nil {
		return nil, fmt.Errorf("list pipelines: %w", err)
	}

	var failures []*JobFailure
	for _, p := range pipelines {
		if p.Commit != sha {
			continue
		}
		if p.Status != woodpecker.StatusFailure && p.Status != woodpecker.StatusError {
			continue
		}

		// Fetch full pipeline to get workflows and steps
		full, err := w.client.Pipeline(repoObj.ID, p.Number)
		if err != nil {
			continue
		}

		wpURL := strings.TrimRight(os.Getenv("WOODPECKER_URL"), "/")
		for _, wf := range full.Workflows {
			for _, step := range wf.Children {
				if step.State != woodpecker.StatusFailure && step.State != woodpecker.StatusError {
					continue
				}

				jf := &JobFailure{
					WorkflowName: wf.Name,
					JobName:      step.Name,
					HTMLURL:      fmt.Sprintf("%s/%s/%d", wpURL, repoObj.FullName, p.Number),
				}

				// Fetch step logs
				logs, logErr := w.client.StepLogEntries(repoObj.ID, p.Number, step.ID)
				if logErr == nil && len(logs) > 0 {
					jf.LogTail = formatWoodpeckerLogs(logs, 50)
				}

				failures = append(failures, jf)
			}
		}
	}

	return failures, nil
}

// formatWoodpeckerLogs extracts the last N lines from Woodpecker log entries.
func formatWoodpeckerLogs(entries []*woodpecker.LogEntry, maxLines int) string {
	var lines []string
	for _, entry := range entries {
		// Skip ExitCode, Metadata, Progress — keep only Stdout and Stderr
		if entry.Type > woodpecker.LogEntryStderr {
			continue
		}
		lines = append(lines, string(entry.Data))
	}

	if len(lines) > maxLines {
		lines = lines[len(lines)-maxLines:]
	}
	return strings.Join(lines, "\n")
}
