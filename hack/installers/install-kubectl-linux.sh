#!/bin/bash
# set -eux -o pipefail

# . $(dirname $0)/../tool-versions.sh

# mkdir -p /tmp/kubectl && cd /tmp/kubectl
# curl -LO https://github.com/betterup/kubernetes/releases/download/v${kubectl_version}/kubectl
# chmod +x kubectl
# cp kubectl $BIN/kubectl
# kubectl version --client

set -eux -o pipefail

. $(dirname $0)/../tool-versions.sh

export TARGET_FILE=kubectl_${ARCHITECTURE}_${kubectl_version}

# NOTE: keep the version synced with https://storage.googleapis.com/kubernetes-release/release/stable.txt
[ -e $DOWNLOADS/${TARGET_FILE} ] || curl -sLf --retry 3 -o ${DOWNLOADS}/${TARGET_FILE} https://storage.googleapis.com/kubernetes-release/release/v${kubectl_version}/bin/linux/$ARCHITECTURE/kubectl
# $(dirname $0)/compare-chksum.sh
sudo install -m 0755 ${DOWNLOADS}/${TARGET_FILE} $BIN/kubectl
