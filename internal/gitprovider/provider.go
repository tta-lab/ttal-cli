package gitprovider

import (
	"bufio"
	"io"
	"net/http"
	"strings"
	"time"
)

type ProviderType string

const (
	ProviderForgejo ProviderType = "forgejo"
	ProviderGitHub  ProviderType = "github"
)

// CI status state constants.
const (
	StatePending = "pending"
	StateSuccess = "success"
	StateFailure = "failure"
	StateError   = "error"
)

type PullRequest struct {
	Index     int64
	Title     string
	Body      string
	State     string
	HTMLURL   string
	Head      string
	HeadSHA   string
	Base      string
	Mergeable bool
	Merged    bool
}

type Comment struct {
	ID        int64
	Body      string
	User      string
	CreatedAt time.Time
	HTMLURL   string
}

// CommitStatus represents the status of a single CI check on a commit.
type CommitStatus struct {
	Context     string // Check name (e.g. "ci/woodpecker", "lint")
	State       string // "pending", "success", "error", "failure"
	Description string
	TargetURL   string
}

// CombinedStatus represents the overall status of all checks on a commit.
type CombinedStatus struct {
	State    string // Overall: "pending", "success", "error", "failure"
	Statuses []*CommitStatus
}

// JobFailure describes a single failed CI job with optional log tail.
type JobFailure struct {
	WorkflowName string
	JobName      string
	LogTail      string // last ~50 lines of job log (best-effort)
	HTMLURL      string
}

type Provider interface {
	Name() string
	CreatePR(owner, repo, head, base, title, body string) (*PullRequest, error)
	FindPR(owner, repo, head, base string) (*PullRequest, error)
	FindPRByState(owner, repo, head, base, state string) (*PullRequest, error)
	EditPR(owner, repo string, index int64, title, body string) (*PullRequest, error)
	GetPR(owner, repo string, index int64) (*PullRequest, error)
	MergePR(owner, repo string, index int64, deleteBranch bool) error
	CreateComment(owner, repo string, index int64, body string) (*Comment, error)
	ListComments(owner, repo string, index int64) ([]*Comment, error)
	GetCombinedStatus(owner, repo, ref string) (*CombinedStatus, error)
	GetCIFailureDetails(owner, repo, sha string, tailLines int) ([]*JobFailure, error)
}

var httpClient = &http.Client{Timeout: 30 * time.Second}

func isFailedStatus(s string) bool {
	return s == StateFailure || s == StateError
}

func tailString(s string, n int) string {
	return tailReader(strings.NewReader(s), n)
}

func fetchLogTail(url string, lines int) string {
	resp, err := httpClient.Get(url) //nolint:gosec
	if err != nil {
		return ""
	}
	defer resp.Body.Close()
	return tailReader(resp.Body, lines)
}

func tailReader(r io.Reader, maxLines int) string {
	if maxLines <= 0 {
		return ""
	}

	recent := make([]string, 0, maxLines)
	reader := bufio.NewReader(r)
	for {
		line, err := reader.ReadString('\n')
		if line != "" {
			line = strings.TrimRight(line, "\r\n")
			if len(recent) == maxLines {
				copy(recent, recent[1:])
				recent[len(recent)-1] = line
			} else {
				recent = append(recent, line)
			}
		}
		if err != nil {
			break
		}
	}

	return strings.Join(recent, "\n")
}
