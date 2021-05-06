#!/bin/bash
set -eux -o pipefail

. $(dirname $0)/../tool-versions.sh

export TARGET_FILE=helm-v${helm2_version}-linux-${ARCHITECTURE}.tar.gz

[ -e ${DOWNLOADS}/${TARGET_FILE} ] || curl -sLf --retry 3 -o ${DOWNLOADS}/${TARGET_FILE} https://storage.googleapis.com/kubernetes-helm/helm-v${helm2_version}-linux-$ARCHITECTURE.tar.gz
$(dirname $0)/compare-chksum.sh
mkdir -p /tmp/helm2 && tar -C /tmp/helm2 -xf $DOWNLOADS/${TARGET_FILE}
sudo install -m 0755 /tmp/helm2/linux-$ARCHITECTURE/helm $BIN/helm2
helm2 version --client
