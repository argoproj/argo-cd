#!/bin/bash
set -eux -o pipefail

export DOWNLOADS=/tmp/dl
export BIN=${BIN:-/go/bin}

mkdir -p $DOWNLOADS

for product in $*; do
  "$(dirname $0)/installers/install-${product}.sh"
done
