#!/bin/bash
set -eux -o pipefail

. $(dirname $0)/../tool-versions.sh

[ -e $DOWNLOADS/helm.tar.gz ] || curl -sLf --retry 3 -o $DOWNLOADS/helm.tar.gz https://get.helm.sh/helm-v${helm3_version}-linux-$ARCHITECTURE.tar.gz
mkdir -p /tmp/helm && tar -C /tmp/helm -xf $DOWNLOADS/helm.tar.gz
cp /tmp/helm/linux-$ARCHITECTURE/helm $BIN/helm
helm version --client
