package pr

import (
	"errors"
	"testing"

	"github.com/tta-lab/ttal-cli/internal/gitprovider"
	"github.com/tta-lab/ttal-cli/internal/taskwarrior"
)

func stubTaskHexFromCwd(t *testing.T, fn func(cwd string) string) func() {
	t.Helper()
	orig := taskHexFromCwdFn
	taskHexFromCwdFn = fn
	return func() { taskHexFromCwdFn = orig }
}

func stubExportTaskByHexID(t *testing.T, fn func(hexID, status string) (*taskwarrior.Task, error)) func() {
	t.Helper()
	orig := exportTaskByHexIDFn
	exportTaskByHexIDFn = fn
	return func() { exportTaskByHexIDFn = orig }
}

func stubDetectProvider(t *testing.T, fn func(workDir string) (*gitprovider.RepoInfo, error)) func() {
	t.Helper()
	orig := detectProviderFn
	detectProviderFn = fn
	return func() { detectProviderFn = orig }
}

func stubResolveProjectPathOrError(t *testing.T, fn func(alias string) (string, error)) func() {
	t.Helper()
	orig := resolveProjectPathOrErrorFn
	resolveProjectPathOrErrorFn = fn
	return func() { resolveProjectPathOrErrorFn = orig }
}

func TestResolveContextWithoutProvider_HexMatchTaskFound(t *testing.T) {
	defer stubTaskHexFromCwd(t, func(cwd string) string {
		return "abc12345"
	})()
	defer stubExportTaskByHexID(t, func(hexID, status string) (*taskwarrior.Task, error) {
		return &taskwarrior.Task{UUID: "abc12345-0000-0000-0000-000000000000", Project: "ttal"}, nil
	})()
	defer stubResolveProjectPathOrError(t, func(alias string) (string, error) {
		return "/projects/ttal", nil
	})()
	defer stubDetectProvider(t, func(workDir string) (*gitprovider.RepoInfo, error) {
		return &gitprovider.RepoInfo{Owner: "tta-lab", Repo: "ttal-cli"}, nil
	})()

	ctx, err := ResolveContextWithoutProvider()
	if err != nil {
		t.Fatalf("ResolveContextWithoutProvider() = _, %v", err)
	}
	if ctx.Task.UUID != "abc12345-0000-0000-0000-000000000000" {
		t.Errorf("Task.UUID = %q, want %q", ctx.Task.UUID, "abc12345-0000-0000-0000-000000000000")
	}
	if ctx.Owner != "tta-lab" {
		t.Errorf("Owner = %q, want %q", ctx.Owner, "tta-lab")
	}
}

func TestResolveContextWithoutProvider_HexMatchTaskNotFound(t *testing.T) {
	defer stubTaskHexFromCwd(t, func(cwd string) string {
		return "abc12345"
	})()
	defer stubExportTaskByHexID(t, func(hexID, status string) (*taskwarrior.Task, error) {
		return nil, errors.New("task not found")
	})()
	defer stubDetectProvider(t, func(workDir string) (*gitprovider.RepoInfo, error) {
		return &gitprovider.RepoInfo{Owner: "tta-lab", Repo: "ttal-cli"}, nil
	})()

	ctx, err := ResolveContextWithoutProvider()
	if err != nil {
		t.Fatalf("ResolveContextWithoutProvider() = _, %v", err)
	}
	if ctx.Task.UUID != "" {
		t.Errorf("Task.UUID = %q, want empty (cwd-only fallback)", ctx.Task.UUID)
	}
}

func TestResolveContextWithoutProvider_NoHexMatch(t *testing.T) {
	defer stubTaskHexFromCwd(t, func(cwd string) string {
		return ""
	})()
	defer stubDetectProvider(t, func(workDir string) (*gitprovider.RepoInfo, error) {
		return &gitprovider.RepoInfo{Owner: "user", Repo: "project"}, nil
	})()

	ctx, err := ResolveContextWithoutProvider()
	if err != nil {
		t.Fatalf("ResolveContextWithoutProvider() = _, %v", err)
	}
	if ctx.Owner != "user" || ctx.Repo != "project" {
		t.Errorf("ctx = %+v, want Owner=user Repo=project", ctx)
	}
}
