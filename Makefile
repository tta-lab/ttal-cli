.PHONY: help build clean clean-db reset test generate install run fmt vet lint

# Default target
help:
	@echo "Available commands:"
	@echo "  make build         - Build the ttal binary"
	@echo "  make install       - Install ttal to GOPATH/bin"
	@echo "  make run           - Run ttal (usage: make run ARGS='project list')"
	@echo "  make clean         - Remove built binaries"
	@echo "  make clean-db      - Remove database (destructive!)"
	@echo "  make reset         - Remove binaries and database (destructive!)"
	@echo "  make test          - Run tests"
	@echo "  make generate      - Regenerate ent code from schemas"
	@echo "  make fmt           - Format code with gofmt"
	@echo "  make vet           - Run go vet"
	@echo "  make lint          - Run golangci-lint (if installed)"
	@echo "  make all           - Format, generate, vet, and build"
	@echo "  make ci            - Run all CI checks (fmt, generate, vet, lint, test, build)"
	@echo "  make check-clean   - Check if working directory is clean"
	@echo "  make install-hooks - Install git pre-commit hook"

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

# Run the CLI
run: build
	@./ttal $(ARGS)

# Clean build artifacts
clean:
	@echo "Cleaning build artifacts..."
	@rm -f ttal
	@echo "✓ Cleaned build artifacts"

# Clean database (destructive!)
clean-db:
	@echo "⚠️  WARNING: This will delete your database!"
	@rm -rf ~/.ttal/ttal.db
	@echo "✓ Database removed"

# Reset everything (destructive!)
reset: clean clean-db
	@echo "✓ Full reset complete"

# Run tests
test:
	@echo "Running tests..."
	@go test -v ./...

# Generate ent code from schemas
generate:
	@echo "Regenerating ent code..."
	@go mod tidy
	@(cd ent && go generate .)
	@echo "✓ ent code regenerated"

# Format code
fmt:
	@echo "Formatting code..."
	@gofmt -w -s .
	@echo "✓ Code formatted"

# Run go vet
vet:
	@echo "Running go vet..."
	@go vet ./...
	@echo "✓ Vet complete"

# Run golangci-lint (if installed)
lint:
	@if command -v golangci-lint >/dev/null 2>&1; then \
		echo "Running golangci-lint..."; \
		golangci-lint run ./...; \
		echo "✓ Lint complete"; \
	else \
		echo "⚠ golangci-lint not installed. Install: https://golangci-lint.run/usage/install/"; \
	fi

# Run all checks and build
all: fmt generate vet build
	@echo "✓ All checks passed and binary built"

# Development workflow
dev: all
	@echo "✓ Development build complete"

# CI target - runs all checks (same as all but exits on failure)
ci: fmt generate vet lint test build
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

# Install pre-commit hook
install-hooks:
	@echo "Installing pre-commit hook..."
	@mkdir -p $$(git rev-parse --git-path hooks)
	@cp -f scripts/pre-commit.sh $$(git rev-parse --git-path hooks)/pre-commit
	@chmod +x $$(git rev-parse --git-path hooks)/pre-commit
	@echo "✓ Pre-commit hook installed at $$(git rev-parse --git-path hooks)/pre-commit"
