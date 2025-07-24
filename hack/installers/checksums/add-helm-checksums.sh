#!/usr/bin/env sh

# Usage: ./add-helm-checksums.sh 3.9.4  # use the desired version

set -e
for arch in amd64 arm64 ppc64le s390x; do
  wget "https://get.helm.sh/helm-v$1-linux-$arch.tar.gz.sha256sum"   -O "helm-v$1-linux-$arch.tar.gz.sha256"
done

for arch in amd64 arm64; do
  wget "https://get.helm.sh/helm-v$1-darwin-$arch.tar.gz.sha256sum"   -O "helm-v$1-darwin-$arch.tar.gz.sha256"
done