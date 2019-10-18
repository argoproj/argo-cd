#!/bin/bash
set -eux -o pipefail

# TODO we use v2 for generating manifests, v3 for production - we should always use v3
KUSTOMIZE_VERSION=${KUSTOMIZE_VERSION:-3.1.0}
DL=$DOWNLOADS/kustomize-${KUSTOMIZE_VERSION}

[ -e $DL ] || curl -sLf --retry 3 -o $DL https://github.com/kubernetes-sigs/kustomize/releases/download/v${KUSTOMIZE_VERSION}/kustomize_${KUSTOMIZE_VERSION}_linux_amd64
cp $DL $BIN/kustomize
chmod +x $BIN/kustomize
kustomize version
