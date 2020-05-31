#!/bin/bash
set -eux -o pipefail

[ -e $DOWNLOADS/swagger ] || curl -sLf --retry 3 -o $DOWNLOADS/swagger https://github.com/go-swagger/go-swagger/releases/download/v0.19.0/swagger_linux_$ARCHITECTURE
cp $DOWNLOADS/swagger $BIN/swagger
chmod +x $BIN/swagger
swagger version
