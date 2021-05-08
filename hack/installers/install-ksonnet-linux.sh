#!/bin/bash
set -eux -o pipefail

. $(dirname $0)/../tool-versions.sh

KSONNET_VERSION=${ksonnet_version}
case $ARCHITECTURE in
  arm|arm64)
    set +o pipefail
    export GO111MODULE=off
    # Clone the repository in $GOPATH/src/github.com/ksonnet/ksonnet
    go get -u github.com/ksonnet/ksonnet/cmd/ks || true
    set -o pipefail
    cd $GOPATH/src/github.com/ksonnet/ksonnet && git checkout tags/v$KSONNET_VERSION
    cd $GOPATH/src/github.com/ksonnet/ksonnet && CGO_ENABLED=0 GO_LDFLAGS="-s" make install
    sudo mv $GOPATH/bin/ks $BIN/ks
    ;;
  *)
    export TARGET_FILE=ks_${ksonnet_version}_linux_${ARCHITECTURE}.tar.gz
    [ -e $DOWNLOADS/${TARGET_FILE} ] || curl -sLf --retry 3 -o $DOWNLOADS/${TARGET_FILE} https://github.com/ksonnet/ksonnet/releases/download/v${KSONNET_VERSION}/ks_${KSONNET_VERSION}_linux_${ARCHITECTURE}.tar.gz
    $(dirname $0)/compare-chksum.sh
    tar -C /tmp -xf $DOWNLOADS/${TARGET_FILE}
    sudo install -m 0755 /tmp/ks_${KSONNET_VERSION}_linux_${ARCHITECTURE}/ks $BIN/ks
    ;;
esac

ks version
