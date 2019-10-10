#!/bin/bash
set -eux -o pipefail

PACKR_VERSION=1.21.9
[ -e $DOWNLOADS/packr.tar.gz ] || curl -sLf --retry 3 -o $DOWNLOADS/packr.tar.gz https://github.com/gobuffalo/packr/releases/download/v${PACKR_VERSION}/packr_${PACKR_VERSION}_linux_amd64.tar.gz
tar -vxf $DOWNLOADS/packr.tar.gz -C /tmp/
mv /tmp/packr $BIN/packr