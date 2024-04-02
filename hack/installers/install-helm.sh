#!/bin/bash
set -eux -o pipefail

. $(dirname $0)/../tool-versions.sh

export TARGET_FILE=helm-v${helm3_version}-${INSTALL_OS}-${ARCHITECTURE}.tar.gz

[ -e $DOWNLOADS/${TARGET_FILE} ] || curl -sLf --retry 3 -o $DOWNLOADS/${TARGET_FILE} https://get.helm.sh/helm-v${helm3_version}-$INSTALL_OS-$ARCHITECTURE.tar.gz
$(dirname $0)/compare-chksum.sh
mkdir -p /tmp/helm && tar -C /tmp/helm -xf $DOWNLOADS/${TARGET_FILE}
sudo install -m 0755 /tmp/helm/$INSTALL_OS-$ARCHITECTURE/helm $BIN/helm
helm version --client
