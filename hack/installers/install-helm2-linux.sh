#!/bin/bash
set -eux -o pipefail

[ -e $DOWNLOADS/helm2.tar.gz ] || curl -sLf --retry 3 -o $DOWNLOADS/helm2.tar.gz https://storage.googleapis.com/kubernetes-helm/helm-v2.15.2-linux-amd64.tar.gz
mkdir -p /tmp/helm2 && tar -C /tmp/helm2 -xf $DOWNLOADS/helm2.tar.gz
cp /tmp/helm2/linux-amd64/helm $BIN/helm2
helm2 version --client
