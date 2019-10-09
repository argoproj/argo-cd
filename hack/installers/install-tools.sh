#!/bin/bash
set -eux -o pipefail

export DOWNLOADS=${DOWNLOADS:-/tmp/dl}
export BIN=${BIN:-$HOME/bin}

$(dirname $0)/install-kubectl.sh
$(dirname $0)/install-kubectx.sh
$(dirname $0)/install-dep.sh
$(dirname $0)/install-ksonnet.sh
$(dirname $0)/install-helm.sh
$(dirname $0)/install-kustomize.sh
$(dirname $0)/install-golangci-lint.sh
$(dirname $0)/install-goimports.sh
