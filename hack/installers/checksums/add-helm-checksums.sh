#!/usr/bin/env sh

# Usage: ./add-helm-checksums.sh 3.9.4  # use the desired version

set -e

wget "https://get.helm.sh/helm-v$1-linux-amd64.tar.gz.sha256sum"   -O "helm-v$1-linux-amd64.tar.gz.sha256"
wget "https://get.helm.sh/helm-v$1-linux-arm64.tar.gz.sha256sum"   -O "helm-v$1-linux-arm64.tar.gz.sha256"
wget "https://get.helm.sh/helm-v$1-linux-ppc64le.tar.gz.sha256sum" -O "helm-v$1-linux-ppc64le.tar.gz.sha256"
wget "https://get.helm.sh/helm-v$1-linux-s390x.tar.gz.sha256sum"   -O "helm-v$1-linux-s390x.tar.gz.sha256"
