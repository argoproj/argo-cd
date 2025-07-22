#!/usr/bin/env bash

out="| Argo CD version | Kubernetes versions |\n"
out+="|-----------------|---------------------|\n"

argocd_current_version=$(git rev-parse --abbrev-ref HEAD | sed 's/release-//')
argocd_major_version_num=$(echo "$argocd_current_version" | sed -E 's/\.[0-9]+//')
argocd_minor_version_num=$(echo "$argocd_current_version" | sed -E 's/[0-9]+\.//')

for _ in {1..3}; do
  argocd_version="${argocd_major_version_num}.${argocd_minor_version_num}"
  git checkout "release-$argocd_version" > /dev/null || exit 1

  line=$(yq '.jobs["test-e2e"].strategy.matrix |
    # k3s-version was an array prior to 2.12. This checks for the old format first and then falls back to the new format.
    (.["k3s-version"] // (.k3s | map(.version))) |
    .[]' .github/workflows/ci-build.yaml | \
    jq --arg argocd_version "$argocd_version" --raw-input --slurp --raw-output \
    'split("\n")[:-1] | map(sub("\\.[0-9]+$"; "")) | join(", ") | "| \($argocd_version) | \(.) |"')
  out+="$line\n"


  # If we're at minor version 0, there's no further version back in this series. Instead, move to the latest version in
  # the previous major release series.
  if [ "$argocd_minor_version_num" -eq 0 ]; then
    argocd_major_version_num=$((argocd_major_version_num - 1))
    # Get the latest minor version in the previous series.
    argocd_minor_version_num=$(git tag -l "v$argocd_major_version_num.*" | sort -V | tail -n 1 | sed -E 's/\.[0-9]+$//' | sed -E 's/^v[0-9]+\.//')
  else
    argocd_minor_version_num=$((argocd_minor_version_num - 1))
  fi
done

git checkout "release-$argocd_current_version"

printf "$out" > docs/operator-manual/tested-kubernetes-versions.md
