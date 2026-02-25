package gitprovider

import "time"

type ProviderType string

const (
	ProviderForgejo ProviderType = "forgejo"
	ProviderGitHub  ProviderType = "github"
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

type Provider interface {
	CreatePR(owner, repo, head, base, title, body string) (*PullRequest, error)
	EditPR(owner, repo string, index int64, title, body string) (*PullRequest, error)
	GetPR(owner, repo string, index int64) (*PullRequest, error)
	MergePR(owner, repo string, index int64, deleteBranch bool) error
	CreateComment(owner, repo string, index int64, body string) (*Comment, error)
	ListComments(owner, repo string, index int64) ([]*Comment, error)
	GetCombinedStatus(owner, repo, ref string) (*CombinedStatus, error)
}
