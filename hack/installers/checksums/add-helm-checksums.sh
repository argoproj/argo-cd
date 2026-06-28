#!/usr/bin/env sh

# Usage: ./add-helm-checksums.sh <helm-version>  # use the desired version e.g. 3.18.4

set -e

for arch in amd64 arm64 ppc64le s390x; do
  checksumfile="helm-v$1-linux-$arch.tar.gz.sha256"
  curl -sLf --retry 3 -o "$checksumfile" "https://get.helm.sh/helm-v$1-linux-$arch.tar.gz.sha256sum"
  outname="$(git rev-parse --show-toplevel)/hack/installers/checksums/helm-v$1-linux-$arch.tar.gz.sha256"
  mv "$checksumfile" "$outname"
done

for arch in amd64 arm64; do
  checksumfile="helm-v$1-darwin-$arch.tar.gz.sha256"
  curl -sLf --retry 3 -o "$checksumfile" "https://get.helm.sh/helm-v$1-darwin-$arch.tar.gz.sha256sum"
  outname="$(git rev-parse --show-toplevel)/hack/installers/checksums/helm-v$1-darwin-$arch.tar.gz.sha256"
  mv "$checksumfile" "$outname"
done
