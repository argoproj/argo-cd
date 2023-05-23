#!/bin/bash
set -eux -o pipefail

. $(dirname $0)/../tool-versions.sh

export TARGET_FILE=kubectl_${ARCHITECTURE}_${kubectl_version}

mkdir -p /tmp/kubectl && cd /tmp/kubectl
curl -LO https://github.com/betterup/kubernetes/releases/download/v1.25.10-1%2Bdee9f04a2c7a62-dirty/kubectl
chmod +x kubectl
cp kubectl $BIN/kubectl
kubectl version --client
