package cmd

import (
	"bytes"
	"errors"
	"strings"
	"testing"

	"github.com/tta-lab/ttal-cli/internal/daemon"
	"github.com/tta-lab/ttal-cli/internal/gitprovider"
	"github.com/tta-lab/ttal-cli/internal/pr"
	"github.com/tta-lab/ttal-cli/internal/taskwarrior"
)

const testHeadSHA = "abc123"

var errTestHeadSHA = errors.New("git rev-parse failed")

func TestPullCmd_MainPullsDefaultBranch(t *testing.T) {
	defer stubPullContext(t, defaultBranchName, &pr.Context{
		Task:  &taskwarrior.Task{},
		Info:  &gitprovider.RepoInfo{Owner: testPROwner, Repo: testPRRepo, DefaultBranch: defaultBranchName},
		Alias: testPRAlias,
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

	if got.Branch != defaultBranchName || got.DefaultBranch != defaultBranchName || got.Mode != daemon.GitPullModeDefault {
		t.Fatalf("request = %+v, want main/default pull", got)
	}
	if !strings.Contains(out.String(), "Pulled main") {
		t.Fatalf("output = %q, want pulled message", out.String())
	}
}

func TestPullCmd_OpenPRPullsCurrentBranch(t *testing.T) {
	defer stubPullContext(t, "feature/x", &pr.Context{
		Task:  &taskwarrior.Task{},
		Owner: testPROwner,
		Repo:  testPRRepo,
		Info: &gitprovider.RepoInfo{
			Owner: testPROwner, Repo: testPRRepo, DefaultBranch: defaultBranchName, Provider: gitprovider.ProviderGitHub,
		},
		Alias: testPRAlias,
	})()

	defer stubDaemonPRFindForPull(t, func(req daemon.PRFindRequest) (daemon.PRFindResponse, error) {
		return daemon.PRFindResponse{OK: true, PRIndex: 12}, nil
	})()
	defer stubCurrentBranchHeadSHA(t, func(workDir, branch string) (string, error) {
		return testHeadSHA, nil
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
	if !strings.Contains(out.String(), "PR for feature/x is not merged") {
		t.Fatalf("output = %q, want unmerged PR plan", out.String())
	}
}

func TestPullCmd_NoPRFoundPullsCurrentBranchWithMessage(t *testing.T) {
	defer stubPullContext(t, "feature/no-pr", &pr.Context{
		Task:  &taskwarrior.Task{},
		Owner: testPROwner,
		Repo:  testPRRepo,
		Info: &gitprovider.RepoInfo{
			Owner: testPROwner, Repo: testPRRepo, DefaultBranch: defaultBranchName, Provider: gitprovider.ProviderGitHub,
		},
		Alias: testPRAlias,
	})()
	defer stubCurrentBranchHeadSHA(t, func(workDir, branch string) (string, error) {
		return testHeadSHA, nil
	})()
	defer stubDaemonPRFindForPull(t, func(req daemon.PRFindRequest) (daemon.PRFindResponse, error) {
		return daemon.PRFindResponse{OK: false, Error: "find PR: no PR found for commit abc123"}, nil
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
	if got.Mode != daemon.GitPullModeBranch {
		t.Fatalf("request = %+v, want branch pull", got)
	}
	if !strings.Contains(out.String(), "No PR found for feature/no-pr") {
		t.Fatalf("output = %q, want no-PR plan", out.String())
	}
}

func TestPullCmd_MergedPRCleansBranch(t *testing.T) {
	defer stubPullContext(t, "feature/x", &pr.Context{
		Task:  &taskwarrior.Task{PRID: "34"},
		Owner: testPROwner,
		Repo:  testPRRepo,
		Info: &gitprovider.RepoInfo{
			Owner: testPROwner, Repo: testPRRepo, DefaultBranch: defaultBranchName, Provider: gitprovider.ProviderGitHub,
		},
		Alias: testPRAlias,
	})()
	defer stubCurrentBranchHeadSHA(t, func(workDir, branch string) (string, error) {
		return testHeadSHA, nil
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

	if got.Mode != daemon.GitPullModeCleanupMerged || got.Branch != "feature/x" || got.DefaultBranch != defaultBranchName {
		t.Fatalf("request = %+v, want merged cleanup", got)
	}
	if !strings.Contains(out.String(), "Deleted feature/x") {
		t.Fatalf("output = %q, want cleanup message", out.String())
	}
}

func TestPullCmd_PRIDPathInvalidLocalBranchErrorsBeforePRLookup(t *testing.T) {
	defer stubPullContext(t, "feature/bad-ref", &pr.Context{
		Task:  &taskwarrior.Task{PRID: "34"},
		Owner: testPROwner,
		Repo:  testPRRepo,
		Info: &gitprovider.RepoInfo{
			Owner: testPROwner, Repo: testPRRepo, DefaultBranch: defaultBranchName, Provider: gitprovider.ProviderGitHub,
		},
		Alias: testPRAlias,
	})()
	defer stubCurrentBranchHeadSHA(t, func(workDir, branch string) (string, error) {
		return "", errTestHeadSHA
	})()
	defer stubDaemonPRGetForPull(t, func(req daemon.PRGetPRRequest) (daemon.PRGetPRResponse, error) {
		t.Fatal("PR lookup should not run when local branch preflight fails")
		return daemon.PRGetPRResponse{}, nil
	})()

	var out bytes.Buffer
	pullCmd.SetOut(&out)
	pullCmd.SetErr(&out)
	err := pullCmd.RunE(pullCmd, nil)
	if err == nil {
		t.Fatal("expected invalid local branch to fail")
	}
	if !strings.Contains(err.Error(), "cannot verify local branch feature/bad-ref") {
		t.Fatalf("error = %q, want local branch preflight message", err)
	}
}

func TestPullCmd_ForgejoPreflightsLocalBranchButUsesBranchLookup(t *testing.T) {
	defer stubPullContext(t, "feature/forgejo", &pr.Context{
		Task:  &taskwarrior.Task{},
		Owner: testPROwner,
		Repo:  testPRRepo,
		Info: &gitprovider.RepoInfo{
			Owner: testPROwner, Repo: testPRRepo, DefaultBranch: defaultBranchName, Provider: gitprovider.ProviderForgejo,
		},
		Alias: testPRAlias,
	})()
	defer stubCurrentBranchHeadSHA(t, func(workDir, branch string) (string, error) {
		if branch != "feature/forgejo" {
			t.Errorf("branch = %q", branch)
		}
		return testHeadSHA, nil
	})()
	defer stubDaemonPRFindForPull(t, func(req daemon.PRFindRequest) (daemon.PRFindResponse, error) {
		if req.HeadSHA != "" {
			t.Errorf("HeadSHA = %q, want empty for Forgejo branch lookup", req.HeadSHA)
		}
		return daemon.PRFindResponse{OK: true, PRIndex: 58, Merged: false}, nil
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
	if got.Mode != daemon.GitPullModeBranch {
		t.Fatalf("request = %+v, want branch pull", got)
	}
}

func TestPullCmd_MergedPRFoundByBranchHeadSHACleansBranch(t *testing.T) {
	defer stubPullContext(t, "feature/deleted-remote", &pr.Context{
		Task:  &taskwarrior.Task{},
		Owner: testPROwner,
		Repo:  testPRRepo,
		Info: &gitprovider.RepoInfo{
			Owner: testPROwner, Repo: testPRRepo, DefaultBranch: defaultBranchName, Provider: gitprovider.ProviderGitHub,
		},
		Alias: testPRAlias,
	})()
	defer stubCurrentBranchHeadSHA(t, func(workDir, branch string) (string, error) {
		if branch != "feature/deleted-remote" {
			t.Errorf("branch = %q", branch)
		}
		return testHeadSHA, nil
	})()
	defer stubDaemonPRFindForPull(t, func(req daemon.PRFindRequest) (daemon.PRFindResponse, error) {
		if req.HeadSHA != testHeadSHA {
			t.Errorf("HeadSHA = %q, want %s", req.HeadSHA, testHeadSHA)
		}
		return daemon.PRFindResponse{OK: true, PRIndex: 56, Merged: true}, nil
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

	if got.Mode != daemon.GitPullModeCleanupMerged {
		t.Fatalf("request = %+v, want cleanup mode", got)
	}
}

func TestPullCmd_InvalidLocalBranchErrorsBeforePRLookup(t *testing.T) {
	defer stubPullContext(t, "feature/bad-ref", &pr.Context{
		Task:  &taskwarrior.Task{},
		Owner: testPROwner,
		Repo:  testPRRepo,
		Info: &gitprovider.RepoInfo{
			Owner: testPROwner, Repo: testPRRepo, DefaultBranch: defaultBranchName, Provider: gitprovider.ProviderForgejo,
		},
		Alias: testPRAlias,
	})()
	defer stubCurrentBranchHeadSHA(t, func(workDir, branch string) (string, error) {
		return "", errTestHeadSHA
	})()

	var out bytes.Buffer
	pullCmd.SetOut(&out)
	pullCmd.SetErr(&out)
	err := pullCmd.RunE(pullCmd, nil)
	if err == nil {
		t.Fatal("expected missing HEAD SHA to fail")
	}
	if !strings.Contains(err.Error(), "cannot verify local branch feature/bad-ref") {
		t.Fatalf("error = %q, want clear HEAD SHA message", err)
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

func stubCurrentBranchHeadSHA(t *testing.T, fn func(workDir, branch string) (string, error)) func() {
	t.Helper()
	orig := currentBranchHeadSHAFn
	currentBranchHeadSHAFn = fn
	return func() { currentBranchHeadSHAFn = orig }
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
