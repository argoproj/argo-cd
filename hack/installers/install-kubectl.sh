#!/bin/bash
set -eux -o pipefail

[ -e $DOWNLOADS/kubectl ] || curl -sLf -C - -o /tmp/dl/kubectl https://storage.googleapis.com/kubernetes-release/release/v1.14.0/bin/linux/amd64/kubectl
cp $DOWNLOADS/kubectl $BIN/kubectl
chmod +x $BIN/kubectl
