#!/bin/bash
set -eux -o pipefail

PACKR_VERSION=1.21.9
[ -e $DOWNLOADS/parkr.tar.gz ] || curl -sLf --retry 3 -o $DOWNLOADS/parkr.tar.gz https://github.com/gobuffalo/packr/releases/download/v${PACKR_VERSION}/packr_${PACKR_VERSION}_linux_amd64.tar.gz
tar -vxf $DOWNLOADS/parkr.tar.gz -C /tmp/
cp /tmp/packr $BIN/
chmod +x $BIN/packr
packr version
