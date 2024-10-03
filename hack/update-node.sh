#!/usr/bin/env bash

# This script is used to update the node version in the project.
# We use this because Dependabot doesn't support updating the Node version in all the places we use Node.

set -e

echo "Getting latest Node version..."

# Get the current LTS node version. This assumes the JSON is sorted newest-to-oldest.
NODE_VERSION=$(curl -s https://nodejs.org/download/release/index.json | jq '.[0].version' -r)

# Make sure the version number is semver with a preceding 'v'.
if [[ ! "$NODE_VERSION" =~ ^v[0-9]+\.[0-9]+\.[0-9]+$ ]]; then
  echo "Failed to get the latest Node version."
  exit 1
fi

# Strip the preceding 'v' from the version number.
NODE_VERSION=${NODE_VERSION#v}

# Get the manifest SHA of the library/node image.
DIGEST=$(crane digest "docker.io/library/node:$NODE_VERSION")

echo "Updating to Node version $NODE_VERSION with digest $DIGEST..."

# Replace the node image in the Dockerfiles.
sed -r -i.bak "s/docker\.io\/library\/node:[0-9.]+@sha256:[0-9a-f]+/docker.io\/library\/node:$NODE_VERSION@$DIGEST/" Dockerfile ui-test/Dockerfile test/container/Dockerfile
rm Dockerfile.bak ui-test/Dockerfile.bak test/container/Dockerfile.bak

# Replace node version in ci-build.yaml.
sed -r -i.bak "s/node-version: '[0-9.]+'/node-version: '$NODE_VERSION'/" .github/workflows/ci-build.yaml
rm .github/workflows/ci-build.yaml.bak
