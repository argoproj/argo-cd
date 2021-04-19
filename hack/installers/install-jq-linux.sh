#!/bin/bash
set -eux -o pipefail

. $(dirname $0)/../tool-versions.sh

export TARGET_FILE=jq-${jq_version}-linux-amd64

[ -e $DOWNLOADS/${TARGET_FILE} ] || curl -sLf --retry 3 -o $DOWNLOADS/${TARGET_FILE} https://github.com/stedolan/jq/releases/download/jq-${jq_version}/jq-linux64
$(dirname $0)/compare-chksum.sh
sudo install -m 0755 $DOWNLOADS/${TARGET_FILE} $BIN/jq
jq --version