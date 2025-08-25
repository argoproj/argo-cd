#!/bin/bash

# Script to insert two lines in Dockerfile after line 110

set -e

DOCKERFILE="Dockerfile"
TEMP_FILE="Dockerfile.tmp"

# Check if Dockerfile exists
if [ ! -f "$DOCKERFILE" ]; then
    echo "Error: $DOCKERFILE not found in current directory"
    exit 1
fi

echo "Processing $DOCKERFILE..."

# Insert the lines after line 110
{
    head -n 110 "$DOCKERFILE"
    echo "RUN mkdir -p gitops-engine"
    echo "COPY gitops-engine/go.* ./gitops-engine"
    tail -n +111 "$DOCKERFILE"
} > "$TEMP_FILE"

# Replace original with modified version
mv "$TEMP_FILE" "$DOCKERFILE"

echo "Successfully inserted lines after line 110:"
echo "  RUN mkdir -p gitops-engine"
echo "  COPY gitops-engine/go.* ./gitops-engine"
echo
echo "Changes applied to $DOCKERFILE"
