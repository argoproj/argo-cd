#!/bin/bash
set -eux -o pipefail

[ -e $BIN/dep ] || curl -sLf -o $BIN/dep https://github.com/golang/dep/releases/download/v0.5.3/dep-linux-amd64
chmod +x $BIN/dep
dep version
