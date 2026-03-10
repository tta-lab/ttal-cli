package main

import (
	"context"
	"flag"
	"fmt"
	"os"

	"dagger.io/dagger"
	"dagger.io/dagger/dag"
)

const imageRef = "ghcr.io/tta-lab/ttal-worker-go"

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
		_, err := container.Export(ctx, "./ttal-worker-go.tar")
		if err != nil {
			return fmt.Errorf("export failed: %w", err)
		}
		fmt.Println("Exported: ttal-worker-go.tar")
		fmt.Println("Load with: docker load -i ttal-worker-go.tar")
	}

	return nil
}

// workerImage builds the Go worker image on top of ttal-base.
func workerImage() *dagger.Container {
	return dag.Container().
		From("ghcr.io/tta-lab/ttal-base:latest").
		// All go install as agent — binaries go to ~/go/bin (already in PATH)
		WithExec([]string{"go", "install", "github.com/golangci/golangci-lint/cmd/golangci-lint@latest"}).
		WithExec([]string{"go", "install", "github.com/goreleaser/goreleaser/v2@latest"}).
		WithExec([]string{"go", "install", "github.com/evilmartians/lefthook@latest"})
}
