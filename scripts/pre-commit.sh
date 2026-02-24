#!/bin/bash
set -e

echo "🔍 Running pre-commit checks..."

# Check if this is an initial commit
if git rev-parse --verify HEAD >/dev/null 2>&1; then
    against=HEAD
else
    # Initial commit: diff against an empty tree object
    against=$(git hash-object -t tree /dev/null)
fi

# Stash unstaged changes to avoid checking uncommitted work
git stash -q --keep-index

# Function to restore stash on exit
restore_stash() {
    if git stash list | grep -q "stash@{0}"; then
        git stash pop -q
    fi
}
trap restore_stash EXIT

# Format check
echo "  📝 Checking formatting..."
make fmt > /dev/null 2>&1
if ! git diff --exit-code --quiet; then
    echo "  ❌ Code not formatted. Applying format..."
    git add -u
fi

# Generate ent code
echo "  🔧 Regenerating ent code..."
make generate > /dev/null 2>&1

# Vet
echo "  🔬 Running go vet..."
if ! make vet > /dev/null 2>&1; then
    echo "  ❌ go vet failed"
    exit 1
fi

# Lint
echo "  🔍 Running lint..."
if ! make lint > /dev/null 2>&1; then
    echo "  ❌ Lint failed"
    make lint 2>&1 | tail -10
    exit 1
fi

# Run tests
echo "  🧪 Running tests..."
if ! make test > /dev/null 2>&1; then
    echo "  ❌ Tests failed"
    exit 1
fi

echo "✅ Pre-commit checks passed"
exit 0
