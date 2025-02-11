#!/usr/bin/env sh

# Usage: ./add-kubectl-checksums.sh v1.32.1  # use the desired version

set -e
for arch in amd64 arm64 ppc64le s390x; do
  wget "https://dl.k8s.io/release/$1/bin/linux/$arch/kubectl.sha256" -O "kubectl-$1-linux-$arch.tar.gz.sha256"

done

for arch in amd64 arm64; do
  wget "https://dl.k8s.io/release/$1/bin/darwin/$arch/kubectl.sha256" -O "kubectl-$1-darwin-$arch.tar.gz.sha256"
done