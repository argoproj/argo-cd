#!/usr/bin/env sh

# Usage: ./add-kustomize-checksums.sh 4.5.7  # use the desired version

set -e

CHECKSUMS_DIR="$(git rev-parse --show-toplevel)/hack/installers/checksums"
tmpdir=$(mktemp -d)
trap 'rm -rf "$tmpdir"' EXIT

curl -sLf --retry 3 -o "$tmpdir/checksums.txt" \
  "https://github.com/kubernetes-sigs/kustomize/releases/download/kustomize%2Fv$1/checksums.txt"

grep -v windows "$tmpdir/checksums.txt" | while read -r line; do
  line=$(echo "$line" | sed "s#v$1#$1#")
  filename=$(echo "$line" | awk '{print $2}')
  echo "$line" > "$CHECKSUMS_DIR/${filename}.sha256"
done
