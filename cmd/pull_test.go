package cmd

import (
	"bytes"
	"strings"
	"testing"

	"github.com/tta-lab/ttal-cli/internal/daemon"
	"github.com/tta-lab/ttal-cli/internal/gitprovider"
	"github.com/tta-lab/ttal-cli/internal/pr"
	"github.com/tta-lab/ttal-cli/internal/taskwarrior"
)

func TestPullCmd_MainPullsDefaultBranch(t *testing.T) {
	defer stubPullContext(t, "main", &pr.Context{
		Task:  &taskwarrior.Task{},
		Info:  &gitprovider.RepoInfo{Owner: "owner", Repo: "repo", DefaultBranch: "main"},
		Alias: "ttal",
	})()

	var got daemon.GitPullRequest
	defer stubGitPull(t, func(req daemon.GitPullRequest) (daemon.GitPullResponse, error) {
		got = req
		return daemon.GitPullResponse{OK: true, Action: daemon.GitPullActionPulledDefault}, nil
	})()

	var out bytes.Buffer
	pullCmd.SetOut(&out)
	pullCmd.SetErr(&out)
	err := pullCmd.RunE(pullCmd, nil)
	if err != nil {
		t.Fatalf("RunE: %v", err)
	}

	if got.Branch != "main" || got.DefaultBranch != "main" || got.Mode != daemon.GitPullModeDefault {
		t.Fatalf("request = %+v, want main/default pull", got)
	}
	if !strings.Contains(out.String(), "Pulled main") {
		t.Fatalf("output = %q, want pulled message", out.String())
	}
}

func TestPullCmd_OpenPRPullsCurrentBranch(t *testing.T) {
	defer stubPullContext(t, "feature/x", &pr.Context{
		Task:  &taskwarrior.Task{},
		Owner: "owner",
		Repo:  "repo",
		Info: &gitprovider.RepoInfo{
			Owner: "owner", Repo: "repo", DefaultBranch: "main", Provider: gitprovider.ProviderGitHub,
		},
		Alias: "ttal",
	})()

	defer stubDaemonPRFindForPull(t, func(req daemon.PRFindRequest) (daemon.PRFindResponse, error) {
		return daemon.PRFindResponse{OK: true, PRIndex: 12}, nil
	})()

	var got daemon.GitPullRequest
	defer stubGitPull(t, func(req daemon.GitPullRequest) (daemon.GitPullResponse, error) {
		got = req
		return daemon.GitPullResponse{OK: true, Action: daemon.GitPullActionPulledBranch}, nil
	})()

	var out bytes.Buffer
	pullCmd.SetOut(&out)
	pullCmd.SetErr(&out)
	if err := pullCmd.RunE(pullCmd, nil); err != nil {
		t.Fatalf("RunE: %v", err)
	}

	if got.Mode != daemon.GitPullModeBranch || got.Branch != "feature/x" {
		t.Fatalf("request = %+v, want branch pull", got)
	}
	if !strings.Contains(out.String(), "Pulled feature/x") {
		t.Fatalf("output = %q, want branch pulled message", out.String())
	}
}

func TestPullCmd_MergedPRCleansBranch(t *testing.T) {
	defer stubPullContext(t, "feature/x", &pr.Context{
		Task:  &taskwarrior.Task{PRID: "34"},
		Owner: "owner",
		Repo:  "repo",
		Info: &gitprovider.RepoInfo{
			Owner: "owner", Repo: "repo", DefaultBranch: "main", Provider: gitprovider.ProviderGitHub,
		},
		Alias: "ttal",
	})()

	defer stubDaemonPRGetForPull(t, func(req daemon.PRGetPRRequest) (daemon.PRGetPRResponse, error) {
		return daemon.PRGetPRResponse{OK: true, Merged: true}, nil
	})()

	var got daemon.GitPullRequest
	defer stubGitPull(t, func(req daemon.GitPullRequest) (daemon.GitPullResponse, error) {
		got = req
		return daemon.GitPullResponse{OK: true, Action: daemon.GitPullActionCleanedMergedBranch}, nil
	})()

	var out bytes.Buffer
	pullCmd.SetOut(&out)
	pullCmd.SetErr(&out)
	if err := pullCmd.RunE(pullCmd, nil); err != nil {
		t.Fatalf("RunE: %v", err)
	}

	if got.Mode != daemon.GitPullModeCleanupMerged || got.Branch != "feature/x" || got.DefaultBranch != "main" {
		t.Fatalf("request = %+v, want merged cleanup", got)
	}
	if !strings.Contains(out.String(), "Deleted feature/x") {
		t.Fatalf("output = %q, want cleanup message", out.String())
	}
}

func stubPullContext(t *testing.T, branch string, ctx *pr.Context) func() {
	t.Helper()
	origResolve := prResolveContextFn
	origBranch := currentBranchFn
	prResolveContextFn = func() (*pr.Context, error) { return ctx, nil }
	currentBranchFn = func(_, _, _ string) string { return branch }
	return func() {
		prResolveContextFn = origResolve
		currentBranchFn = origBranch
	}
}

func stubGitPull(t *testing.T, fn func(daemon.GitPullRequest) (daemon.GitPullResponse, error)) func() {
	t.Helper()
	orig := gitPullFn
	gitPullFn = fn
	return func() { gitPullFn = orig }
}

func stubDaemonPRFindForPull(t *testing.T, fn func(daemon.PRFindRequest) (daemon.PRFindResponse, error)) func() {
	t.Helper()
	orig := daemonPRFindFn
	daemonPRFindFn = fn
	return func() { daemonPRFindFn = orig }
}

func stubDaemonPRGetForPull(t *testing.T, fn func(daemon.PRGetPRRequest) (daemon.PRGetPRResponse, error)) func() {
	t.Helper()
	orig := daemonPRGetPRFn
	daemonPRGetPRFn = fn
	return func() { daemonPRGetPRFn = orig }
}
