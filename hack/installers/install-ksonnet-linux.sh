#!/bin/bash
set -eux -o pipefail

. $(dirname $0)/../tool-versions.sh

KSONNET_VERSION=${ksonnet_version}
case $ARCHITECTURE in
  arm|arm64)
    set +o pipefail
    # Clone the repository in $GOPATH/src/github.com/ksonnet/ksonnet
    go get -u github.com/ksonnet/ksonnet || true
    set -o pipefail
    cd $GOPATH/src/github.com/ksonnet/ksonnet && git checkout tags/v$KSONNET_VERSION
    cd $GOPATH/src/github.com/ksonnet/ksonnet && CGO_ENABLED=0 GO_LDFLAGS="-s" make install
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
