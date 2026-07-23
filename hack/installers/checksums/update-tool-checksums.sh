#!/usr/bin/env sh

# Usage: ./update-tool-checksums.sh <helm|kustomize|git-lfs> <version>

set -e

script_dir=$(cd "$(dirname "$0")" && pwd)

case "$1" in
  helm)
    "$script_dir/add-helm-checksums.sh" "$2"
    ;;
  kustomize)
    "$script_dir/add-kustomize-checksums.sh" "$2"
    ;;
  git-lfs)
    "$script_dir/add-git-lfs-checksums.sh" "$2"
    ;;
  *)
    echo "unknown tool: $1 (expected helm, kustomize, or git-lfs)" >&2
    exit 1
    ;;
esac
