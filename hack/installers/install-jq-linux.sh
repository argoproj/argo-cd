#!/bin/bash
set -eux -o pipefail

. $(dirname $0)/../tool-versions.sh

[ -e $DOWNLOADS/jq ] || curl -sLf --retry 3 -o $DOWNLOADS/jq https://github.com/stedolan/jq/releases/download/jq-${jq_version}/jq-linux64
cp $DOWNLOADS/jq $BIN/jq
chmod +x $BIN/jq
jq --version