#!/bin/bash
set -eux -o pipefail

[ -e $DOWNLOADS/dep ] || curl -sLf -o $DOWNLOADS/dep https://github.com/golang/dep/releases/download/v0.5.3/dep-linux-amd64
sudo cp $DOWNLOADS/dep $BIN/
sudo chmod +x $BIN/dep
dep version
