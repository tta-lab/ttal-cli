package gitprovider

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/google/go-github/v69/github"
)

const testGitHubBaseBranch = "main"

func TestGitHubProviderFindPRByCommit(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/repos/o/r/commits/abc123/pulls", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("method = %s, want GET", r.Method)
		}
		if got := r.URL.Query().Get("per_page"); got != "2" {
			t.Errorf("per_page = %q, want 2", got)
		}
		_, _ = w.Write([]byte(`[{
			"number": 56,
			"title": "fix pull",
			"state": "closed",
			"html_url": "https://github.com/o/r/pull/56",
			"merged_at": "2026-05-24T09:16:33Z",
			"head": {"ref": "feature/deleted-remote", "sha": "abc123"},
			"base": {"ref": "main"}
		}]`))
	})
	server := httptest.NewServer(mux)
	t.Cleanup(server.Close)

	client := github.NewClient(server.Client())
	baseURL, err := url.Parse(server.URL + "/")
	if err != nil {
		t.Fatalf("parse server URL: %v", err)
	}
	client.BaseURL = baseURL
	provider := &GitHubProvider{client: client}

	pr, err := provider.FindPRByCommit("o", "r", "abc123")
	if err != nil {
		t.Fatalf("FindPRByCommit: %v", err)
	}
	if pr.Index != 56 || !pr.Merged || pr.Head != "feature/deleted-remote" || pr.Base != testGitHubBaseBranch {
		t.Fatalf("PR = %+v, want merged PR #56 for feature/deleted-remote -> main", pr)
	}
}

func TestFetchLogTailReadsPastFirst64KiB(t *testing.T) {
	body := strings.Repeat("setup success\n", 6000) + "actual failure\nexit status 1\n"
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(body))
	}))
	t.Cleanup(server.Close)

	got := fetchLogTail(server.URL, 2)
	want := "actual failure\nexit status 1"
	if got != want {
		t.Fatalf("fetchLogTail() = %q, want %q", got, want)
	}
}
