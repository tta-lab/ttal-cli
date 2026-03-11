package main

import (
	"context"
	"flag"
	"fmt"
	"os"

	"dagger.io/dagger"
	"dagger.io/dagger/dag"
)

const imageRef = "ghcr.io/tta-lab/ttal-worker-node"

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

	container := workerImage()

	if push {
		ref := fmt.Sprintf("%s:%s", imageRef, tag)
		published, err := container.Publish(ctx, ref)
		if err != nil {
			return fmt.Errorf("push failed: %w", err)
		}
		fmt.Printf("Pushed: %s\n", published)
	} else {
		_, err := container.Export(ctx, "./ttal-worker-node.tar")
		if err != nil {
			return fmt.Errorf("export failed: %w", err)
		}
		fmt.Println("Exported: ttal-worker-node.tar")
		fmt.Println("Load with: docker load -i ttal-worker-node.tar")
	}

	return nil
}

// workerImage builds the Node worker image on top of ttal-base.
// Proto installs are user-level (~/.proto/). pnpm + biome need root for npm global.
func workerImage() *dagger.Container {
	return dag.Container().
		From("ghcr.io/tta-lab/ttal-base:latest").
		// Proto + version-managed tools (user-level → ~/.proto/)
		WithExec([]string{"bash", "-c", "curl -fsSL https://moonrepo.dev/install/proto.sh | bash"}).
		WithExec([]string{"proto", "install", "bun", "1.3.5"}).
		WithExec([]string{"proto", "install", "moon", "2.0.1"}).
		// npm global installs need root
		WithUser("root").
		WithExec([]string{"npm", "install", "-g", "pnpm", "@biomejs/biome"}).
		WithUser("agent")
}
