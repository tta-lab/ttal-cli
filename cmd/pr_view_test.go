package cmd

import (
	"testing"

	"github.com/tta-lab/ttal-cli/internal/daemon"
	"github.com/tta-lab/ttal-cli/internal/gitprovider"
	"github.com/tta-lab/ttal-cli/internal/pr"
	"github.com/tta-lab/ttal-cli/internal/taskwarrior"
)

func TestPRViewCommandExists(t *testing.T) {
	cmd, _, err := rootCmd.Find([]string{"pr", "view"})
	if err != nil {
		t.Fatalf("pr view command not found: %v", err)
	}
	if cmd.Name() != "view" {
		t.Errorf("expected command name 'view', got %q", cmd.Name())
	}
	if len(prViewCmd.Aliases) != 1 || prViewCmd.Aliases[0] != "list" {
		t.Errorf("expected alias 'list', got %v", prViewCmd.Aliases)
	}
}

func TestPRListAliasWorks(t *testing.T) {
	cmd, _, err := rootCmd.Find([]string{"pr", "list"})
	if err != nil {
		t.Fatalf("pr list alias not found: %v", err)
	}
	if cmd.Name() != "view" {
		t.Errorf("expected alias 'list' to resolve to 'view', got %q", cmd.Name())
	}
}

func TestPRView_SendsStateAll(t *testing.T) {
	defer stubPRResolveContext(t, func() (*pr.Context, error) {
		return &pr.Context{
			Task:  &taskwarrior.Task{},
			Owner: testPROwner,
			Repo:  testPRRepo,
			Info: &gitprovider.RepoInfo{
				Owner: testPROwner, Repo: testPRRepo, DefaultBranch: defaultBranchName,
				Provider: gitprovider.ProviderForgejo,
			},
			Alias: testPRAlias,
		}, nil
	})()
	defer stubCurrentBranch(t, func(uuid, alias, workDir string) string {
		return "feature/test"
	})()
	defer stubCurrentBranchHeadSHA(t, func(workDir, branch string) (string, error) {
		return "abc123", nil
	})()

	var got daemon.PRFindRequest
	defer stubDaemonPRFind(t, func(req daemon.PRFindRequest) (daemon.PRFindResponse, error) {
		got = req
		return daemon.PRFindResponse{OK: true, PRIndex: 1}, nil
	})()
	defer stubDaemonPRGetForPull(t, func(req daemon.PRGetPRRequest) (daemon.PRGetPRResponse, error) {
		return daemon.PRGetPRResponse{OK: true, Title: "test", State: "open"}, nil
	})()

	err := prViewCmd.RunE(prViewCmd, nil)
	if err != nil {
		t.Fatalf("pr view: %v", err)
	}
	if got.State != "all" {
		t.Errorf("PRFindRequest.State = %q, want %q", got.State, "all")
	}
}
