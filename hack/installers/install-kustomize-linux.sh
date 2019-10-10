#!/bin/bash
set -eux -o pipefail

[ -e $DOWNLOADS/kustomize ] || curl -sLf --retry 3 -o $DOWNLOADS/kustomize https://github.com/kubernetes-sigs/kustomize/releases/download/v3.1.0/kustomize_3.1.0_linux_amd64
cp $DOWNLOADS/kustomize $BIN/kustomize
chmod +x $BIN/kustomize
kustomize version
