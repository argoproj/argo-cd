#!/usr/bin/env bash

out="| Argo CD version | Kubernetes versions |\n"
out+="|-----------------|---------------------|\n"

argocd_minor_version=$(git rev-parse --abbrev-ref HEAD | sed 's/release-//')
argocd_major_version_num=$(echo "$argocd_minor_version" | sed -E 's/\.[0-9]+//')
argocd_minor_version_num=$(echo "$argocd_minor_version" | sed -E 's/[0-9]+\.//')

for n in 0 1 2; do
  minor_version_num=$((argocd_minor_version_num - n))
  minor_version="${argocd_major_version_num}.${minor_version_num}"
  git checkout "release-$minor_version" > /dev/null || exit 1
  line=$(yq '.jobs["test-e2e"].strategy.matrix["k3s-version"][]' .github/workflows/ci-build.yaml | \
    jq --arg minor_version "$minor_version" --raw-input --slurp --raw-output \
    'split("\n")[:-1] | map(sub("\\.[0-9]+$"; "")) | join(", ") | "| \($minor_version) | \(.) |"')
  out+="$line\n"
done

git checkout "release-$argocd_minor_version"


printf "$out" > docs/operator-manual/tested-kubernetes-versions.md
