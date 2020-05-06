#!/bin/bash
set -eux -o pipefail

KSONNET_VERSION=0.13.1
case $ARCHITECTURE in
  arm|arm64)
    set +o pipefail
    go get -u github.com/ksonnet/ksonnet || true
    set -o pipefail
    cd $GOPATH/src/github.com/ksonnet/ksonnet && git checkout tags/v$KSONNET_VERSION
    cd $GOPATH/src/github.com/ksonnet/ksonnet && make install
    mv $GOPATH/bin/ks $BIN/ks
    ;;
  *)
    [ -e $DOWNLOADS/ks.tar.gz ] || curl -sLf --retry 3 -o $DOWNLOADS/ks.tar.gz https://github.com/ksonnet/ksonnet/releases/download/v${KSONNET_VERSION}/ks_${KSONNET_VERSION}_linux_${ARCHITECTURE}.tar.gz
    tar -C /tmp -xf $DOWNLOADS/ks.tar.gz
    cp /tmp/ks_${KSONNET_VERSION}_linux_${ARCHITECTURE}/ks $BIN/ks
    ;;
esac

chmod +x $BIN/ks
ks version
