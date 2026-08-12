#!/usr/bin/env sh

# Usage: ./add-git-lfs-checksums.sh 3.7.1  # use the desired version

set -e

CHECKSUMS_DIR="$(git rev-parse --show-toplevel)/hack/installers/checksums"
tmpdir=$(mktemp -d)
trap 'rm -rf "$tmpdir"' EXIT

curl -sLf --retry 3 -o "$tmpdir/sha256sums.asc" \
  "https://github.com/git-lfs/git-lfs/releases/download/v$1/sha256sums.asc"

grep '^\S\{64\} \*git-lfs-linux-' "$tmpdir/sha256sums.asc" | while read -r line; do
  hash=$(echo "$line" | awk '{print $1}')
  filename=$(echo "$line" | awk '{print $2}' | tr -d '*')
  echo "$hash  $filename" > "$CHECKSUMS_DIR/${filename}.sha256"
done
