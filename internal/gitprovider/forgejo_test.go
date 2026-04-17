package gitprovider

import (
	"testing"
)

func TestNewForgejoProvider_EmptyHost(t *testing.T) {
	_, err := NewForgejoProvider("")
	if err == nil {
		t.Error("expected error for empty host")
	}
}

func TestNewForgejoProvider_MissingToken(t *testing.T) {
	t.Setenv("FORGEJO_TOKEN", "")
	t.Setenv("FORGEJO_ACCESS_TOKEN", "")

	_, err := NewForgejoProvider("git.example.com")
	if err == nil {
		t.Error("expected error for missing token")
	}
}
