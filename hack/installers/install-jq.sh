#!/bin/bash
set -eux -o pipefail

[ -e $DOWNLOADS/jq ] || curl -sLf --retry 3 -o $DOWNLOADS/jq https://github.com/stedolan/jq/releases/download/jq-1.6/jq-linux64
sudo cp /tmp/jq $BIN/jq
sudo chmod +x $BIN/jq
jq --version