#!/bin/bash
set -eux -o pipefail

. $(dirname $0)/../tool-versions.sh

mkdir -p /tmp/kubectl && cd /tmp/kubectl
curl -LO https://github.com/betterup/kubernetes/releases/download/v${kubectl_version}/kubectl
chmod +x kubectl
cp kubectl $BIN/kubectl
kubectl version --client
