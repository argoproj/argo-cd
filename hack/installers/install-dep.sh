#!/bin/bash
set -eux -o pipefail

if [ $(uname -s) = Darwin ]; then
  [ -e $BIN/dep ] || curl -sLf -C - -o $BIN/dep https://github.com/golang/dep/releases/download/v0.5.3/dep-darwin-amd64
else
  [ -e $BIN/dep ] || curl -sLf -C - -o $BIN/dep https://github.com/golang/dep/releases/download/v0.5.3/dep-linux-amd64
fi
chmod +x $BIN/dep
dep version
