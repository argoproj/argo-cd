#!/bin/bash

# Script to replace all occurrences of "go work vendor" with "go work vendor" recursively

set -e

go work init ./gitops-engine
go work use .
go work vendor
go mod tidy

echo "Searching for files containing 'go work vendor'..."

# Replace in each file
for file in Makefile Tiltfile hack/generate-proto.sh .github/workflows/bump-major-version.yaml .github/workflows/ci-build.yaml; do
    echo "Processing: $file"
    # Create a backup and replace
    sed -i.bak 's/go mod vendor/go work vendor/g' "$file"
    echo "  - Replaced occurrences in $file"
    echo "  - Backup created: $file.bak"
done

echo
echo "Replacement complete!"
echo "Removing backup files..."
find . -name '*.bak' -delete

echo "Syncing go mod files..."
go work sync
go mod tidy
go work vendor

