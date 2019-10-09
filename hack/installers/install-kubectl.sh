#!/bin/bash
set -eux -o pipefail

[ -e $DOWNLOADS/kubectl ] || curl -sLf -o $DOWNLOADS/kubectl https://storage.googleapis.com/kubernetes-release/release/v1.14.0/bin/linux/amd64/kubectl
sudo cp $DOWNLOADS/kubectl $BIN/
sudo chmod +x $BIN/kubectl
