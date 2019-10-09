#!/bin/bash
set -eux -o pipefail

[ -e $DOWNLOADS/swagger ] || curl -sLf --retry 5 -o $DOWNLOADS/swagger https://github.com/go-swagger/go-swagger/releases/download/v0.19.0/swagger_linux_amd64
sudo cp $DOWNLOADS/swagger /usr/local/bin/swagger
sudo chmod +x /usr/local/bin/swagger
swagger version
