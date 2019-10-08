#!/bin/bash
set -eux -o pipefail

[ -e $DOWNLOADS/dep ] || curl -sLf -C - -o /tmp/dl/dep https://github.com/golang/dep/releases/download/v0.5.3/dep-linux-amd64
cp $DOWNLOADS/dep $BIN/dep
chmod +x $BIN/dep
dep version
