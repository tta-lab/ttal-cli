package main

import (
	"context"
	"flag"
	"fmt"
	"os"

	"dagger.io/dagger"
	"dagger.io/dagger/dag"
)

const imageRef = "ghcr.io/tta-lab/ttal-base"

const (
	// helixReleaseTag is the GitHub release tag — check https://github.com/helix-editor/helix/releases for updates.
	helixReleaseTag = "25.07.1"
	// helixDebVersion is the Debian package version for the same release.
	// Debian version strings strip leading zeros from numeric components (25.07.1 → 25.7.1).
	helixDebVersion = "25.7.1"
	// goVersion is the Go version to install in the base image — keep in sync with go.mod.
	goVersion = "1.26.0"
)

// agentPath is the PATH for the agent user — includes user-level bin dirs before system dirs.
const agentPath = "/home/agent/.local/bin:/home/agent/go/bin:/home/agent/.cargo/bin:" +
	"/home/agent/.bun/bin:/home/agent/.proto/shims:/home/agent/.proto/bin:" +
	"/usr/local/go/bin:/usr/local/bin:/usr/bin:/bin"

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

	container := base(ctx)

	if push {
		ref := fmt.Sprintf("%s:%s", imageRef, tag)
		published, err := container.Publish(ctx, ref)
		if err != nil {
			return fmt.Errorf("push failed: %w", err)
		}
		fmt.Printf("Pushed: %s\n", published)
	} else {
		_, err := container.Export(ctx, "./ttal-base.tar")
		if err != nil {
			return fmt.Errorf("export failed: %w", err)
		}
		fmt.Println("Exported: ttal-base.tar")
		fmt.Println("Load with: docker load -i ttal-base.tar")
	}

	return nil
}

// base builds the fat base image with all runtimes and tools.
func base(_ context.Context) *dagger.Container {
	// Stage 1: Build ttal binary from source (linux/amd64).
	// This avoids the Mach-O problem where a macOS host binary cannot run in Linux containers.
	ttalBinary := dag.Container().
		From("golang:"+goVersion).
		WithDirectory("/src", dag.Host().Directory("../../", dagger.HostDirectoryOpts{
			Exclude: []string{".git", "containers"},
		})).
		WithWorkdir("/src").
		WithEnvVariable("CGO_ENABLED", "0").
		WithExec([]string{"go", "build", "-o", "/ttal", "."}).
		File("/ttal")

	// Stage 2: Fat base image with all runtimes + tools.
	base := dag.Container().
		From("node:22-bookworm-slim").
		WithExec([]string{"apt-get", "update"}).
		WithExec([]string{"apt-get", "install", "-y", "--no-install-recommends",
			// Core utilities
			"curl", "git", "ssh", "jq", "unzip", "ca-certificates", "bash",
			"build-essential", "pkg-config", "libcurl4-openssl-dev",
			// Session & editing
			"tmux", "vim",
			// Search & find
			"ripgrep", "fd-find",
			// Task management
			"taskwarrior",
			// Scripting
			"python3",
		}).
		WithExec([]string{"bash", "-c", "apt-get clean && rm -rf /var/lib/apt/lists/*"}).

		// Helix editor — .deb from GitHub releases (not in Debian repos)
		WithExec([]string{"sh", "-c", fmt.Sprintf(
			"curl -fsSL https://github.com/helix-editor/helix/releases/download/%s/helix_%s-1_$(dpkg --print-architecture).deb -o /tmp/helix.deb"+
				" && dpkg -i /tmp/helix.deb && rm /tmp/helix.deb",
			helixReleaseTag, helixDebVersion)}).

		// Go toolchain — temporary container to extract /usr/local/go (no spawned process, cross-image copy).
		// Note: goVersion is also used in Stage 1 (golang:+goVersion builder). Both reference the same image;
		// if a registry/tag failure occurs, check both stages.
		WithDirectory("/usr/local/go", dag.Container().From("golang:"+goVersion).Directory("/usr/local/go")).

		// Claude Code + OpenCode (npm global — runs as root for system-level install)
		WithExec([]string{"npm", "install", "-g", "@anthropic-ai/claude-code", "opencode-ai"}).

		// ttal binary (built from source in Stage 1)
		WithFile("/usr/local/bin/ttal", ttalBinary, dagger.ContainerWithFileOpts{
			Permissions: 0o755,
		}).

		// Create agent user with home directory
		WithExec([]string{"useradd", "-m", "-s", "/bin/bash", "agent"}).
		WithExec([]string{"mkdir", "-p", "/home/agent/.local/bin"}).
		WithExec([]string{"chown", "-R", "agent:agent", "/home/agent"})

	// Switch to agent user for user-level installs (Rust via rustup)
	return base.
		WithUser("agent").
		WithEnvVariable("HOME", "/home/agent").
		WithEnvVariable("GOPATH", "/home/agent/go").
		WithEnvVariable("CARGO_HOME", "/home/agent/.cargo").
		WithEnvVariable("CGO_ENABLED", "0").
		WithEnvVariable("PATH", agentPath).

		// Rust (user-level → ~/.cargo/bin)
		WithExec([]string{"bash", "-c",
			"curl --proto '=https' --tlsv1.2 -sSf https://sh.rustup.rs | sh -s -- -y --default-toolchain stable",
		}).
		WithWorkdir("/workspace")
}
