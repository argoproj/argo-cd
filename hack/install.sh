#!/bin/bash
set -eux -o pipefail

export DOWNLOADS=/tmp/dl
export BIN=${BIN:-/usr/local/bin}

mkdir -p $DOWNLOADS

for product in $*; do
  "$(dirname $0)/installers/install-${product}.sh"
done
