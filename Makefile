.PHONY: help build build-dictate clean clean-projects reset test install install-dictate reinstall setup run fmt lint doc-dev doc-build doc-deploy

# Default target
help:
	@echo "Available commands:"
	@echo "  make build         - Build the ttal binary"
	@echo "  make install       - Install ttal to GOPATH/bin"
	@echo "  make build-dictate - Build with voice dictation (macOS, requires portaudio)"
	@echo "  make install-dictate - Install with voice dictation (macOS, requires portaudio)"
	@echo "  make reinstall     - Install and restart daemon"
	@echo "  make setup         - First-time setup (install + hook + daemon)"
	@echo "  make run           - Run ttal (usage: make run ARGS='project list')"
	@echo "  make clean         - Remove built binaries"
	@echo "  make reset         - Remove binaries"
	@echo "  make test          - Run tests"
	@echo "  make fmt           - Format code with gofmt"
	@echo "  make lint          - Run golangci-lint (16 linters)"
	@echo "  make all           - Format, tidy, lint, and build"
	@echo "  make ci            - Run all CI checks (lint, test, build)"
	@echo "  make check-clean   - Check if working directory is clean"
	@echo "  make install-hooks - Install lefthook git hooks"
	@echo "  make doc-dev       - Start docs dev server"
	@echo "  make doc-build     - Build docs site"
	@echo "  make doc-deploy    - Build and deploy docs to Cloudflare"


# Build the binary
build:
	@echo "Building ttal..."
	@go build -o ttal .
	@echo "✓ Build complete: ./ttal"

# Install to GOPATH/bin
install:
	@echo "Installing ttal..."
	@go build -o $(shell go env GOPATH)/bin/ttal .
	@echo "✓ Installed to $(shell go env GOPATH)/bin/ttal"

# Build with voice dictation support (macOS only, requires portaudio)
build-dictate:
	@echo "Building ttal with voice dictation..."
	@go build -tags voice_dictate -o ttal .
	@codesign -s - ./ttal
	@echo "✓ Build complete: ./ttal (signed, dictation enabled)"

# Install with voice dictation support
install-dictate:
	@echo "Installing ttal with voice dictation..."
	@go build -tags voice_dictate -o $(shell go env GOPATH)/bin/ttal .
	@codesign -s - $(shell go env GOPATH)/bin/ttal
	@echo "✓ Installed to $(shell go env GOPATH)/bin/ttal (signed, dictation enabled)"

# First-time setup: install binary, hooks, and daemon
setup: install
	@echo "Setting up ttal..."
	@$(shell go env GOPATH)/bin/ttal daemon install
	@echo "✓ Setup complete"
	@echo ""
	@echo "Next steps:"
	@echo "  1. Run: ttal doctor --fix (installs hooks, generates template .env)"
	@echo "  2. Edit ~/.config/ttal/.env with your bot tokens"

# Install and restart all running daemons
reinstall: install
	@for label in $$(launchctl list 2>/dev/null | awk '/io\.guion\.ttal\.daemon/ {print $$3}'); do \
		echo "Restarting $$label..."; \
		launchctl kickstart -k "gui/$$(id -u)/$$label"; \
		echo "✓ $$label restarted"; \
	done

# Run the CLI
run: build
	@./ttal $(ARGS)

# Clean build artifacts
clean:
	@echo "Cleaning build artifacts..."
	@rm -f ttal
	@echo "✓ Cleaned build artifacts"

# Reset (clean build artifacts)
reset: clean
	@echo "✓ Reset complete"

# Run tests
test:
	@echo "Running tests..."
	@go test -v -count=1 ./...

# Tidy go modules
tidy:
	@echo "Tidying go modules..."
	@go mod tidy
	@echo "✓ go mod tidy complete"

# Format code
fmt:
	@echo "Formatting code..."
	@gofmt -w -s .
	@echo "✓ Code formatted"

# Run golangci-lint
lint:
	@echo "Running golangci-lint..."
	@golangci-lint run ./...
	@echo "✓ Lint complete"

# Run all checks and build
all: fmt tidy lint build
	@echo "✓ All checks passed and binary built"

# Development workflow
dev: all
	@echo "✓ Development build complete"

# CI target - runs all checks (same as all but exits on failure)
ci: lint test build
	@echo "✓ CI checks complete"

# Check if working directory is clean (for CI)
check-clean:
	@if [ -n "$$(git status --porcelain)" ]; then \
		echo "❌ Working directory is not clean"; \
		git status --short; \
		exit 1; \
	else \
		echo "✓ Working directory is clean"; \
	fi

# Documentation site
doc-dev:
	@cd docs && pnpm dev

doc-build:
	@cd docs && pnpm build

doc-deploy:
	@echo "Building and deploying docs..."
	@cd docs && pnpm build && pnpm exec wrangler deploy
	@echo "Docs deployed to ttal.guion.io"

# Install lefthook git hooks (pre-commit: gofmt + goimports, pre-push: golangci-lint + trufflehog)
install-hooks:
	@lefthook install
	@echo "✓ Lefthook hooks installed (pre-commit: gofmt + goimports, pre-push: golangci-lint + trufflehog)"
