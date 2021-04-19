#!/bin/bash
set -eux -o pipefail

. $(dirname $0)/../tool-versions.sh

PACKR_VERSION=${packr_version}
case $ARCHITECTURE in
  arm|arm64)
    go get -d github.com/gobuffalo/packr@v$PACKR_VERSION
    cd $GOPATH/pkg/mod/github.com/gobuffalo/packr@v$PACKR_VERSION && CGO_ENABLED=0 make install
    sudo install -m 0755 $GOPATH/bin/packr $BIN/packr
    ;;
  *)
    export TARGET_FILE=packr_linux_${ARCHITECTURE}_${packr_version}.tar.gz
    [ -e $DOWNLOADS/${TARGET_FILE} ] || curl -sLf --retry 3 -o $DOWNLOADS/${TARGET_FILE} https://github.com/gobuffalo/packr/releases/download/v${PACKR_VERSION}/packr_${PACKR_VERSION}_linux_$ARCHITECTURE.tar.gz
    $(dirname $0)/compare-chksum.sh
    mkdir -p /tmp/packr-${packr_version}
    tar -vxf $DOWNLOADS/${TARGET_FILE} -C /tmp/packr-${packr_version}
    sudo install -m 0755 /tmp/packr-${packr_version}/packr $BIN/packr
    ;;
esac

packr version
