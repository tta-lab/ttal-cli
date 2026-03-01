package git

import (
	"context"
	"fmt"
	"os/exec"
	"strings"
	"time"
)

// FindRoot returns the git repository root for the given path.
// If path is already the git root, returns it unchanged.
// Returns an error if path is not inside a git repository.
func FindRoot(path string) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, "git", "-C", path, "rev-parse", "--show-toplevel")
	out, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("not a git repository %s: %w", path, err)
	}

	return strings.TrimSpace(string(out)), nil
}
