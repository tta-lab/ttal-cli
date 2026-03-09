package main

import (
	"context"
	"flag"
	"fmt"
	"os"

	"dagger.io/dagger/dag"
)

const imageRef = "ghcr.io/tta-lab/ttal-manager-cc"

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
			"git", "tmux", "curl", "wget", "jq", "tree",
			"openssh-client", "ca-certificates",
			"build-essential", "python3",
			"taskwarrior", "fish",
		}).
		WithExec([]string{"apt-get", "clean"}).
		WithExec([]string{"sh", "-c", "rm -rf /var/lib/apt/lists/*"}).

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
