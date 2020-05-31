#!/bin/bash
set -eux -o pipefail

. $(dirname $0)/../tool-versions.sh

[ -e $DOWNLOADS/swagger ] || curl -sLf --retry 3 -o $DOWNLOADS/swagger https://github.com/go-swagger/go-swagger/releases/download/v${swagger_version}/swagger_linux_$ARCHITECTURE
cp $DOWNLOADS/swagger $BIN/swagger
chmod +x $BIN/swagger
swagger version
