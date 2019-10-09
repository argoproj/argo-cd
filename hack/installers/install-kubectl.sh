#!/bin/bash
set -eux -o pipefail

[ -e $BIN/kubectl ] || curl -sLf -o $BIN/kubectl https://storage.googleapis.com/kubernetes-release/release/v1.14.0/bin/linux/amd64/kubectl
chmod +x $BIN/kubectl
