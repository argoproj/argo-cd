#!/bin/bash

# Script to add two lines after the line containing "COPY go.* ./" in Dockerfile
# The first line adds "green" and the second adds "red"

DOCKERFILE="Dockerfile"

# Check if Dockerfile exists
if [[ ! -f "$DOCKERFILE" ]]; then
    echo "Error: $DOCKERFILE not found!"
    exit 1
fi

# Use sed to insert the two lines after the line containing "COPY go.* ./"
# The pattern looks for the line containing "COPY go.* ./" and adds two lines after it
sed -i.tmp '/COPY go\.\* \.\//{
a\
RUN mkdir -p gitops-engine
a\
COPY gitops-engine/go.* ./gitops-engine
}' "$DOCKERFILE"

# Remove the temporary file created by sed
rm "${DOCKERFILE}.tmp" 2>/dev/null || true

echo "Successfully added required lines to $DOCKERFILE:"
echo ""
echo "Lines around the modification:"
grep -n -A 3 -B 1 "COPY go\.\* \./" "$DOCKERFILE"
