package gitprovider

import (
	"testing"
)

func TestParseRemoteURL(t *testing.T) {
	tests := []struct {
		name      string
		url       string
		wantOwner string
		wantRepo  string
		wantHost  string
		wantErr   bool
	}{
		{
			name:      "SSH shorthand",
			url:       "git@github.com:owner/repo.git",
			wantOwner: "owner",
			wantRepo:  "repo",
			wantHost:  "github.com",
			wantErr:   false,
		},
		{
			name:      "SSH shorthand without .git",
			url:       "git@git.guion.io:clawteam/myproject",
			wantOwner: "clawteam",
			wantRepo:  "myproject",
			wantHost:  "git.guion.io",
			wantErr:   false,
		},
		{
			name:      "SSH protocol",
			url:       "ssh://git@github.com/owner/repo.git",
			wantOwner: "owner",
			wantRepo:  "repo",
			wantHost:  "github.com",
			wantErr:   false,
		},
		{
			name:      "SSH protocol with port",
			url:       "ssh://git@git.example.com:2222/owner/repo.git",
			wantOwner: "owner",
			wantRepo:  "repo",
			wantHost:  "git.example.com",
			wantErr:   false,
		},
		{
			name:      "HTTPS URL",
			url:       "https://github.com/owner/repo.git",
			wantOwner: "owner",
			wantRepo:  "repo",
			wantHost:  "github.com",
			wantErr:   false,
		},
		{
			name:      "HTTPS URL without .git",
			url:       "https://git.guion.io/clawteam/project",
			wantOwner: "clawteam",
			wantRepo:  "project",
			wantHost:  "git.guion.io",
			wantErr:   false,
		},
		{
			name:      "HTTP URL",
			url:       "http://git.example.com/owner/repo.git",
			wantOwner: "owner",
			wantRepo:  "repo",
			wantHost:  "git.example.com",
			wantErr:   false,
		},
		{
			name:    "malformed - no slash",
			url:     "git@github.com:justrepo",
			wantErr: true,
		},
		{
			name:    "malformed - empty owner",
			url:     "git@github.com:/repo.git",
			wantErr: true,
		},
		{
			name:    "malformed - empty repo",
			url:     "git@github.com:owner/.git",
			wantErr: true,
		},
		{
			name:    "invalid URL format",
			url:     "not-a-url",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ParseRemoteURL(tt.url)
			if (err != nil) != tt.wantErr {
				t.Errorf("ParseRemoteURL() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if tt.wantErr {
				return
			}
			if got.Owner != tt.wantOwner {
				t.Errorf("ParseRemoteURL().Owner = %v, want %v", got.Owner, tt.wantOwner)
			}
			if got.Repo != tt.wantRepo {
				t.Errorf("ParseRemoteURL().Repo = %v, want %v", got.Repo, tt.wantRepo)
			}
			if got.Host != tt.wantHost {
				t.Errorf("ParseRemoteURL().Host = %v, want %v", got.Host, tt.wantHost)
			}
		})
	}
}

func TestDetectProviderFromHost(t *testing.T) {
	tests := []struct {
		host         string
		wantProvider ProviderType
	}{
		{"github.com", ProviderGitHub},
		{"GitHub.com", ProviderGitHub},
		{"GITHUB.COM", ProviderGitHub},
		{"git.guion.io", ProviderForgejo},
		{"git.example.com", ProviderForgejo},
		{"codeberg.org", ProviderForgejo},
	}

	for _, tt := range tests {
		t.Run(tt.host, func(t *testing.T) {
			got := detectProviderFromHost(tt.host)
			if got != tt.wantProvider {
				t.Errorf("detectProviderFromHost(%q) = %v, want %v", tt.host, got, tt.wantProvider)
			}
		})
	}
}

func TestSplitPath(t *testing.T) {
	tests := []struct {
		path      string
		wantOwner string
		wantRepo  string
		wantErr   bool
	}{
		{"owner/repo", "owner", "repo", false},
		{"owner/repo.git", "owner", "repo", false},
		{"clawteam/my-project", "clawteam", "my-project", false},
		{"", "", "", true},
		{"nogitconfig", "", "", true},
		{"/repo", "", "", true},
		{"owner/", "", "", true},
		{"/repo.git", "", "", true},
	}

	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			owner, repo, err := splitPath(tt.path)
			if (err != nil) != tt.wantErr {
				t.Errorf("splitPath() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if owner != tt.wantOwner || repo != tt.wantRepo {
				t.Errorf("splitPath() = (%v, %v), want (%v, %v)", owner, repo, tt.wantOwner, tt.wantRepo)
			}
		})
	}
}

func TestWebURL(t *testing.T) {
	baseCases := []struct {
		name    string
		repo    *RepoInfo
		wantURL string
	}{
		{
			name:    "GitHub",
			repo:    &RepoInfo{Owner: "tta-lab", Repo: "ttal-cli", Provider: ProviderGitHub, Host: "github.com"},
			wantURL: "https://github.com/tta-lab/ttal-cli",
		},
		{
			name:    "Forgejo without FORGEJO_URL",
			repo:    &RepoInfo{Owner: "clawteam", Repo: "myproject", Provider: ProviderForgejo, Host: "git.guion.io"},
			wantURL: "https://git.guion.io/clawteam/myproject",
		},
	}

	for _, tt := range baseCases {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.repo.WebURL()
			if got != tt.wantURL {
				t.Errorf("WebURL() = %v, want %v", got, tt.wantURL)
			}
		})
	}

	t.Run("Forgejo with FORGEJO_URL", func(t *testing.T) {
		t.Setenv("FORGEJO_URL", "https://internal.example.com")
		repo := &RepoInfo{Owner: "myorg", Repo: "project", Provider: ProviderForgejo, Host: "git.internal.io"}
		got := repo.WebURL()
		want := "https://internal.example.com/myorg/project"
		if got != want {
			t.Errorf("WebURL() = %v, want %v", got, want)
		}
	})
}
