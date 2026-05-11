package daemon

import (
	"errors"
	"sync"
	"testing"

	"github.com/tta-lab/ttal-cli/internal/gitprovider"
	"github.com/tta-lab/ttal-cli/internal/taskwarrior"
	"github.com/tta-lab/ttal-cli/internal/worker"
)

func TestScanTeam_NoPRID_Skips(t *testing.T) {
	origList := listTasksWithPRFn
	origTarget := resolveTmuxTargetFn
	origExists := windowExistsPrWatchFn
	t.Cleanup(func() {
		listTasksWithPRFn = origList
		resolveTmuxTargetFn = origTarget
		windowExistsPrWatchFn = origExists
	})

	// Shouldn't be called since tasks have no PRID
	listTasksWithPRFn = func() ([]taskwarrior.Task, error) { return nil, nil }

	// Simulate a task with no PRID - ListTasksWithPR only returns tasks with PRIDs
	// so scanTeam should see nothing
	active := make(map[string]bool)
	done := make(chan struct{})
	defer close(done)

	seen := scanTeam(nil, "test", &sync.Mutex{}, active, done)
	if len(seen) != 0 {
		t.Errorf("expected no seen UUIDs, got %d", len(seen))
	}
}

func TestScanTeam_OwnerWorkerWindowStartsPolling(t *testing.T) {
	origList := listTasksWithPRFn
	origTarget := resolveTmuxTargetFn
	origExists := windowExistsPrWatchFn
	origParse := parsePRIDFn
	origResolvePath := resolveProjectPathFn
	origDetect := detectProviderFn
	t.Cleanup(func() {
		listTasksWithPRFn = origList
		resolveTmuxTargetFn = origTarget
		windowExistsPrWatchFn = origExists
		parsePRIDFn = origParse
		resolveProjectPathFn = origResolvePath
		detectProviderFn = origDetect
	})

	listTasksWithPRFn = func() ([]taskwarrior.Task, error) {
		return []taskwarrior.Task{{
			UUID:        "abc12345-0000-0000-0000-000000000000",
			Description: "test task",
			Project:     "ttal",
			Owner:       "astra",
			Tags:        []string{"feature"},
			PRID:        "123",
		}}, nil
	}
	resolveTmuxTargetFn = func(task *taskwarrior.Task) (worker.TmuxTarget, error) {
		return worker.TmuxTarget{
			Session: "ttal-default-astra",
			Window:  "coder",
			WorkDir: "/tmp/wt",
		}, nil
	}
	windowExistsPrWatchFn = func(session, window string) bool {
		return session == "ttal-default-astra" && window == "coder" //nolint:goconst
	}
	parsePRIDFn = func(prid string) (taskwarrior.PRIDInfo, error) {
		return taskwarrior.PRIDInfo{Index: 123}, nil
	}
	resolveProjectPathFn = func(alias string) string {
		return "/home/test/ttal"
	}
	detectProviderFn = func(path string) (*gitprovider.RepoInfo, error) {
		return &gitprovider.RepoInfo{
			Owner: "tta-lab", Repo: "ttal-cli", Provider: "github",
			Host: "github.com",
		}, nil
	}

	active := make(map[string]bool)
	done := make(chan struct{})
	// Close done quickly so pollPR returns immediately
	close(done)

	seen := scanTeam(nil, "test", &sync.Mutex{}, active, done)
	if len(seen) != 1 {
		t.Errorf("expected 1 seen UUID, got %d", len(seen))
	}
	if !seen["abc12345-0000-0000-0000-000000000000"] {
		t.Error("expected task UUID to be in seen set")
	}
}

func TestScanTeam_MissingOwnerSkippedFromActive(t *testing.T) {
	origList := listTasksWithPRFn
	origTarget := resolveTmuxTargetFn
	origExists := windowExistsPrWatchFn
	t.Cleanup(func() {
		listTasksWithPRFn = origList
		resolveTmuxTargetFn = origTarget
		windowExistsPrWatchFn = origExists
	})

	listTasksWithPRFn = func() ([]taskwarrior.Task, error) {
		return []taskwarrior.Task{{
			UUID:        "abc12345-0000-0000-0000-000000000000",
			Description: "test task",
			Project:     "ttal",
			Tags:        []string{"feature"},
			PRID:        "123",
		}}, nil
	}
	resolveTmuxTargetFn = func(task *taskwarrior.Task) (worker.TmuxTarget, error) {
		return worker.TmuxTarget{}, errors.New("no owner")
	}
	windowExistsPrWatchFn = func(session, window string) bool { return true }

	active := make(map[string]bool)
	done := make(chan struct{})
	defer close(done)

	_ = scanTeam(nil, "test", &sync.Mutex{}, active, done)
	// scanTeam adds task to seenUUIDs even when skipped (by design, prevents re-polling)
	// but should NOT add to active map
	if active["abc12345-0000-0000-0000-000000000000"] {
		t.Error("task should not be added to active map when resolution fails")
	}
}

func TestScanTeam_MissingWindowSkips(t *testing.T) {
	origList := listTasksWithPRFn
	origTarget := resolveTmuxTargetFn
	origExists := windowExistsPrWatchFn
	t.Cleanup(func() {
		listTasksWithPRFn = origList
		resolveTmuxTargetFn = origTarget
		windowExistsPrWatchFn = origExists
	})

	listTasksWithPRFn = func() ([]taskwarrior.Task, error) {
		return []taskwarrior.Task{{
			UUID:        "abc12345-0000-0000-0000-000000000000",
			Description: "test task",
			Project:     "ttal",
			Owner:       "astra",
			Tags:        []string{"feature"},
			PRID:        "123",
		}}, nil
	}
	resolveTmuxTargetFn = func(task *taskwarrior.Task) (worker.TmuxTarget, error) {
		return worker.TmuxTarget{Session: "ttal-default-astra", Window: "coder", WorkDir: "/tmp/wt"}, nil
	}
	windowExistsPrWatchFn = func(session, window string) bool {
		return false
	}

	active := make(map[string]bool)
	done := make(chan struct{})
	defer close(done)

	_ = scanTeam(nil, "test", &sync.Mutex{}, active, done)
	// Task seen but not added to active when window doesn't exist
	if active["abc12345-0000-0000-0000-000000000000"] {
		t.Error("task should not be added to active map when window missing")
	}
}

func TestScanTeam_TargetUsesAgentName(t *testing.T) {
	origList := listTasksWithPRFn
	origTarget := resolveTmuxTargetFn
	origExists := windowExistsPrWatchFn
	origParse := parsePRIDFn
	origResolvePath := resolveProjectPathFn
	origDetect := detectProviderFn
	t.Cleanup(func() {
		listTasksWithPRFn = origList
		resolveTmuxTargetFn = origTarget
		windowExistsPrWatchFn = origExists
		parsePRIDFn = origParse
		resolveProjectPathFn = origResolvePath
		detectProviderFn = origDetect
	})

	var capturedWindow string
	listTasksWithPRFn = func() ([]taskwarrior.Task, error) {
		return []taskwarrior.Task{{
			UUID:        "abc12345-0000-0000-0000-000000000000",
			Description: "test task",
			Project:     "ttal",
			Owner:       "astra",
			Tags:        []string{"feature"},
			PRID:        "123",
		}}, nil
	}
	resolveTmuxTargetFn = func(task *taskwarrior.Task) (worker.TmuxTarget, error) {
		capturedWindow = "pr-review-lead"
		return worker.TmuxTarget{Session: "ttal-default-astra", Window: "pr-review-lead", WorkDir: "/tmp/wt"}, nil
	}
	windowExistsPrWatchFn = func(session, window string) bool { return true }
	parsePRIDFn = func(prid string) (taskwarrior.PRIDInfo, error) {
		return taskwarrior.PRIDInfo{Index: 123}, nil
	}
	resolveProjectPathFn = func(alias string) string { return "/home/test/ttal" }
	detectProviderFn = func(path string) (*gitprovider.RepoInfo, error) {
		return &gitprovider.RepoInfo{Owner: "tta-lab", Repo: "ttal-cli", Provider: "github", Host: "github.com"}, nil
	}

	active := make(map[string]bool)
	done := make(chan struct{})
	close(done)

	_ = scanTeam(nil, "test", &sync.Mutex{}, active, done)
	if capturedWindow != "pr-review-lead" {
		t.Errorf("window = %q, want agent name 'pr-review-lead'", capturedWindow)
	}
}
