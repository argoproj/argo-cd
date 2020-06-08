#!/bin/bash
set -eux -o pipefail

[ -e $DOWNLOADS/dep ] || curl -sLf --retry 3 -o $DOWNLOADS/dep https://github.com/golang/dep/releases/download/v0.5.3/dep-linux-$ARCHITECTURE
cp $DOWNLOADS/dep $BIN/
chmod +x $BIN/dep
dep version
