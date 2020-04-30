#!/bin/bash
set -eux -o pipefail

[ -e $DOWNLOADS/helm.tar.gz ] || curl -sLf --retry 3 -o $DOWNLOADS/helm.tar.gz https://get.helm.sh/helm-v3.2.0-linux-amd64.tar.gz
mkdir -p /tmp/helm && tar -C /tmp/helm -xf $DOWNLOADS/helm.tar.gz
cp /tmp/helm/linux-amd64/helm $BIN/helm
helm version --client
