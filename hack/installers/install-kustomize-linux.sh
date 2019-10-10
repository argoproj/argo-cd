#!/bin/bash
set -eux -o pipefail

export VER=3.1.0
[ -e $DOWNLOADS/kustomize_${VER} ] || curl -sLf --retry 3 -o $DOWNLOADS/kustomize_${VER} https://github.com/kubernetes-sigs/kustomize/releases/download/v${VER}/kustomize_${VER}_linux_amd64
cp $DOWNLOADS/kustomize_${VER} $BIN/kustomize
chmod +x $BIN/kustomize
kustomize version
