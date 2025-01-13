#!/usr/bin/env bash

# This script bumps the major version of Argo CD. Before cutting a new major release, run this script and open a PR.

# Get the current version from go.mod.
CURRENT_VERSION=$(grep 'module github.com/argoproj/argo-cd' go.mod | awk '{print $2}' | sed 's/.*\/v//')

echo "Upgrading from v${CURRENT_VERSION} to v$((CURRENT_VERSION + 1))..."

# sed commands in this script use -i.bak for compatibility with both GNU sed and BSD sed.

for file in .golangci.yaml .goreleaser.yaml .mockery.yaml Makefile Procfile; do
  echo "Incrementing the major version in $file..."
  sed -i.bak "s/github\.com\/argoproj\/argo-cd\/v${CURRENT_VERSION}/github\.com\/argoproj\/argo-cd\/v$((CURRENT_VERSION + 1))/g" "$file" && echo "  Updated $file" && rm "$file.bak"
done

for file in hack/generate-proto.sh hack/update-codegen.sh hack/update-openapi.sh; do
  echo "Incrementing the major version in $file..."
  sed -i.bak "s/v${CURRENT_VERSION}/v$((CURRENT_VERSION + 1))/g" "$file" && echo "  Update $file" && rm "$file.bak"
done

echo "Incrementing the major version in proto files..."
find . -name '*.proto' -not -path "./vendor/*" -not -path "./dist/*" -exec sed -i.bak "s/github\.com\/argoproj\/argo-cd\/v${CURRENT_VERSION}/github\.com\/argoproj\/argo-cd\/v$((CURRENT_VERSION + 1))/g" {} \; -exec echo "  Updated" {} \; -exec rm {}.bak \;
find . -name '*.proto' -not -path "./vendor/*" -not -path "./dist/*" -exec sed -i.bak "s/github\.com\.argoproj\.argo_cd\.v${CURRENT_VERSION}/github\.com\.argoproj\.argo_cd\.v$((CURRENT_VERSION + 1))/g" {} \; -exec echo "  Updated" {} \; -exec rm {}.bak \;

echo "Incrementing the major version in go files..."
find . -name '*.go' -not -path "./vendor/*" -not -path "./dist/*" -exec sed -i.bak "s/github\.com\/argoproj\/argo-cd\/v${CURRENT_VERSION}/github\.com\/argoproj\/argo-cd\/v$((CURRENT_VERSION + 1))/g" {} \; -exec echo "  Updated" {} \; -exec rm {}.bak \;

echo "Incrementing the major version in go.mod..."
sed -i.bak "s/github\.com\/argoproj\/argo-cd\/v${CURRENT_VERSION}/github\.com\/argoproj\/argo-cd\/v$((CURRENT_VERSION + 1))/g" go.mod && echo "  Updated go.mod" && rm go.mod.bak

echo 'Finished! Now run `make codegen-local && make lint-local && make test-local` to ensure everything is working as expected.'
