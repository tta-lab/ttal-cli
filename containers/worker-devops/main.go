package main

import (
	"context"
	"flag"
	"fmt"
	"os"

	"dagger.io/dagger"
	"dagger.io/dagger/dag"
)

const imageRef = "ghcr.io/tta-lab/ttal-worker-devops"

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
		_, err := container.Export(ctx, "./ttal-worker-devops.tar")
		if err != nil {
			return fmt.Errorf("export failed: %w", err)
		}
		fmt.Println("Exported: ttal-worker-devops.tar")
		fmt.Println("Load with: docker load -i ttal-worker-devops.tar")
	}

	return nil
}

// workerImage builds the DevOps worker image on top of ttal-base.
// go install and cargo install run as agent. Binary downloads to /usr/local/bin/ need root.
func workerImage() *dagger.Container {
	return dag.Container().
		From("ghcr.io/tta-lab/ttal-base:latest").
		// Go tools as agent — binaries go to ~/go/bin (already in PATH)
		WithExec([]string{"go", "install", "github.com/grafana/tanka/cmd/tk@latest"}).
		WithExec([]string{"go", "install", "github.com/jsonnet-bundler/jsonnet-bundler/cmd/jb@latest"}).
		// Bun (user-level → ~/.bun/bin, already in PATH)
		WithExec([]string{"bash", "-c", "curl -fsSL https://bun.sh/install | bash"}).
		// System binaries need root for /usr/local/bin/ installs
		WithUser("root").
		WithExec([]string{"bash", "-c",
			"curl -LO https://dl.k8s.io/release/v1.33.0/bin/linux/amd64/kubectl && chmod +x kubectl && mv kubectl /usr/local/bin/",
		}).
		WithExec([]string{"bash", "-c",
			"curl https://raw.githubusercontent.com/helm/helm/main/scripts/get-helm-3 | bash",
		}).
		WithExec([]string{"bash", "-c",
			"curl -LO https://github.com/getsops/sops/releases/latest/download/sops-v3-linux-amd64 && chmod +x sops-v3-linux-amd64 && mv sops-v3-linux-amd64 /usr/local/bin/sops",
		}).
		WithExec([]string{"bash", "-c",
			"curl -LO https://github.com/FiloSottile/age/releases/latest/download/age-linux-amd64.tar.gz && tar -xzf age-linux-amd64.tar.gz && mv age/age* /usr/local/bin/ && rm -rf age age-linux-amd64.tar.gz",
		}).
		WithExec([]string{"bash", "-c",
			"curl -fsSL https://sh.vector.dev | bash -s -- -y",
		}).
		WithUser("agent")
}
