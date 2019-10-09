#!/bin/bash
set -eux -o pipefail

[ -e $DOWNLOADS/ks.tar.gz ] || curl -sLf -o $DOWNLOADS/ks.tar.gz https://github.com/ksonnet/ksonnet/releases/download/v0.13.1/ks_0.13.1_linux_amd64.tar.gz
tar -C /tmp -xf $DOWNLOADS/ks.tar.gz
sudo cp /tmp/ks_0.13.1_linux_amd64/ks $BIN/ks
sudo chmod +x $BIN/ks
ks version
