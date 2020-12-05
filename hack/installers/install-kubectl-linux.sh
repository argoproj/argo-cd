#!/bin/bash
set -eux -o pipefail

. $(dirname $0)/../tool-versions.sh

# NOTE: keep the version synced with https://storage.googleapis.com/kubernetes-release/release/stable.txt
[ -e $DOWNLOADS/kubectl ] || curl -sLf --retry 3 -o $DOWNLOADS/kubectl https://storage.googleapis.com/kubernetes-release/release/v${kubectl_version}/bin/linux/$ARCHITECTURE/kubectl
cp $DOWNLOADS/kubectl $BIN/
chmod +x $BIN/kubectl
