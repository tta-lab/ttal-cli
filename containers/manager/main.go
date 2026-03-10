package main

import (
	"context"
	"flag"
	"fmt"
	"os"

	"dagger.io/dagger/dag"
)

const imageRef = "ghcr.io/tta-lab/ttal-manager-cc"

const (
	// helixReleaseTag is the GitHub release tag — check https://github.com/helix-editor/helix/releases for updates.
	helixReleaseTag = "25.07.1"
	// helixDebVersion is the Debian package version for the same release.
	// Debian version strings strip leading zeros from numeric components (25.07.1 → 25.7.1).
	helixDebVersion = "25.7.1"
)

func main() {
	push := flag.Bool("push", false, "push image to GHCR")
	tag := flag.String("tag", "latest", "image tag")
	flag.Parse()

	if err := build(context.Background(), *push, *tag); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func build(ctx context.Context, push bool, tag string) error {
	defer func() {
		if err := dag.Close(); err != nil {
			fmt.Fprintf(os.Stderr, "warning: dag.Close: %v\n", err)
		}
	}()

	if push && tag == "" {
		return fmt.Errorf("--tag must not be empty when --push is set")
	}

	container := dag.Container().
		From("node:22-slim").

		// System packages
		WithExec([]string{"apt-get", "update"}).
		WithExec([]string{"apt-get", "install", "-y", "--no-install-recommends",
			"git", "tmux", "curl", "jq",
			"openssh-client", "ca-certificates",
			"build-essential", "python3",
			"taskwarrior", "ripgrep", "fd-find",
			"vim",
		}).

		// GitHub CLI (not in standard Debian repos — add official apt source)
		WithExec([]string{"sh", "-c", "curl -fsSL https://cli.github.com/packages/githubcli-archive-keyring.gpg | dd of=/usr/share/keyrings/githubcli-archive-keyring.gpg"}).
		WithExec([]string{"sh", "-c", `echo "deb [arch=$(dpkg --print-architecture) signed-by=/usr/share/keyrings/githubcli-archive-keyring.gpg] https://cli.github.com/packages stable main" | tee /etc/apt/sources.list.d/github-cli.list`}).
		WithExec([]string{"apt-get", "update"}).
		WithExec([]string{"apt-get", "install", "-y", "--no-install-recommends", "gh"}).
		WithExec([]string{"apt-get", "clean"}).
		WithExec([]string{"sh", "-c", "rm -rf /var/lib/apt/lists/*"}).

		// Helix editor — .deb from GitHub releases (arch-aware, matches pattern used for gh above)
		WithExec([]string{"sh", "-c", fmt.Sprintf(
			"curl -fsSL https://github.com/helix-editor/helix/releases/download/%s/helix_%s-1_$(dpkg --print-architecture).deb -o /tmp/helix.deb"+
				" && dpkg -i /tmp/helix.deb && rm /tmp/helix.deb",
			helixReleaseTag, helixDebVersion)}).
		WithExec([]string{"hx", "--version"}).

		// Claude Code via npm (latest, no pin)
		WithExec([]string{"npm", "install", "-g", "@anthropic-ai/claude-code"}).

		// Non-root user
		WithUser("node").
		WithWorkdir("/workspace")

	if push {
		ref := fmt.Sprintf("%s:%s", imageRef, tag)
		published, err := container.Publish(ctx, ref)
		if err != nil {
			return fmt.Errorf("push failed: %w", err)
		}
		fmt.Printf("Pushed: %s\n", published)
	} else {
		_, err := container.Export(ctx, "./ttal-manager-cc.tar")
		if err != nil {
			return fmt.Errorf("export failed: %w", err)
		}
		fmt.Println("Exported: ttal-manager-cc.tar")
		fmt.Println("Load with: podman load -i ttal-manager-cc.tar")
	}

	return nil
}
