#!/usr/bin/env bash

# This script is used to update the Go version in the project.
# We use this because Dependabot doesn't support updating the Go version in all the places we use Go.

set -e

echo "Getting latest Go version..."

# Get the current stable Go version. This assumes the JSON is sorted newest-to-oldest.
GO_VERSION=$(curl -s https://go.dev/dl/?mode=json | jq 'map(select(.stable == true))[0].version' -r)

# Make sure the version number is semver.
if [[ ! "$GO_VERSION" =~ ^go[0-9]+\.[0-9]+\.[0-9]+$ ]]; then
  echo "Failed to get the latest Go version."
  exit 1
fi

# Remove the 'go' prefix from the version number.
GO_VERSION=${GO_VERSION#go}

# Get the digest of the Go image.
DIGEST=$(crane digest "docker.io/library/golang:$GO_VERSION")

echo "Updating to Go version $GO_VERSION with digest $DIGEST..."

# Replace the Go image in the Dockerfile.
sed -r -i.bak "s/docker\.io\/library\/golang:[0-9.]+@sha256:[0-9a-f]+/docker.io\/library\/golang:$GO_VERSION@$DIGEST/" Dockerfile test/container/Dockerfile test/remote/Dockerfile
rm Dockerfile.bak test/container/Dockerfile.bak test/remote/Dockerfile.bak

# Update the go version in ci-build.yaml, image.yaml, and release.yaml.
sed -r -i.bak "s/go-version: [0-9.]+/go-version: $GO_VERSION/" .github/workflows/ci-build.yaml .github/workflows/image.yaml .github/workflows/release.yaml
rm .github/workflows/ci-build.yaml.bak .github/workflows/image.yaml.bak .github/workflows/release.yaml.bak

# Repeat for env var instead of go-version.
sed -r -i.bak "s/GOLANG_VERSION: '[0-9.]+'/GOLANG_VERSION: '$GO_VERSION'/" .github/workflows/ci-build.yaml .github/workflows/image.yaml .github/workflows/release.yaml
rm .github/workflows/ci-build.yaml.bak .github/workflows/image.yaml.bak .github/workflows/release.yaml.bak

