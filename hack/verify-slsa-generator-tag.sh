#!/usr/bin/env bash
# Verify that a tag exists on slsa-framework/slsa-github-generator and resolves to a commit.
# Usage: verify-slsa-generator-tag.sh <tag>
# Optional: GITHUB_TOKEN for higher GitHub API rate limits (set automatically in Actions).

set -euo pipefail

usage() {
  echo "Usage: $0 <tag>" >&2
  echo "  Example: $0 v2.1.0" >&2
  exit 1
}

[[ $# -eq 1 ]] || usage
tag=$1

api() {
  local curl_args=(
    -fsSL
    -H 'Accept: application/vnd.github+json'
    -H 'X-GitHub-Api-Version: 2022-11-28'
  )
  if [[ -n "${GITHUB_TOKEN:-}" ]]; then
    curl_args+=(-H "Authorization: Bearer ${GITHUB_TOKEN}")
  fi
  curl "${curl_args[@]}" "$1"
}

echo "Verifying tag: ${tag}"

ref_json=$(api "https://api.github.com/repos/slsa-framework/slsa-github-generator/git/ref/tags/${tag}")
otype=$(jq -r '.object.type' <<<"$ref_json")
case "$otype" in
  commit)
    sha=$(jq -r '.object.sha' <<<"$ref_json")
    ;;
  tag)
    tag_url=$(jq -r '.object.url' <<<"$ref_json")
    tag_json=$(api "$tag_url")
    inner_type=$(jq -r '.object.type' <<<"$tag_json")
    if [[ "$inner_type" != "commit" ]]; then
      echo "unexpected peeled type: ${inner_type}" >&2
      exit 1
    fi
    sha=$(jq -r '.object.sha' <<<"$tag_json")
    ;;
  *)
    echo "unexpected object type: ${otype}" >&2
    exit 1
    ;;
esac

if [[ ! "$sha" =~ ^[0-9a-f]{40}$ ]]; then
  echo "resolved SHA is not a full commit hash: ${sha}" >&2
  exit 1
fi

echo "Tag ${tag} resolves to commit ${sha}"
echo OK
