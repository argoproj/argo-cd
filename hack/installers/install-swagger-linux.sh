#!/bin/bash
set -eux -o pipefail

. $(dirname $0)/../tool-versions.sh

export TARGET_FILE=swagger_linux_${ARCHITECTURE}_${swagger_version}
[ -e $DOWNLOADS/${TARGET_FILE} ] || curl -sLf --retry 3 -o $DOWNLOADS/${TARGET_FILE} https://github.com/go-swagger/go-swagger/releases/download/v${swagger_version}/swagger_linux_$ARCHITECTURE
$(dirname $0)/compare-chksum.sh
sudo install -m 0755 $DOWNLOADS/${TARGET_FILE} $BIN/swagger
swagger version
