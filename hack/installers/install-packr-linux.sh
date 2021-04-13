#!/bin/bash
set -eux -o pipefail

. $(dirname $0)/../tool-versions.sh

PACKR_VERSION=${packr_version}
case $ARCHITECTURE in
  arm|arm64)
    go get -d github.com/gobuffalo/packr@v$PACKR_VERSION
    cd $GOPATH/pkg/mod/github.com/gobuffalo/packr@v$PACKR_VERSION && CGO_ENABLED=0 make install
    mv $GOPATH/bin/packr $BIN/packr
    ;;
  *)
    [ -e $DOWNLOADS/parkr.tar.gz ] || curl -sLf --retry 3 -o $DOWNLOADS/parkr.tar.gz https://github.com/gobuffalo/packr/releases/download/v${PACKR_VERSION}/packr_${PACKR_VERSION}_linux_$ARCHITECTURE.tar.gz
    tar -vxf $DOWNLOADS/parkr.tar.gz -C /tmp/
    cp /tmp/packr $BIN/
    ;;
esac

chmod +x $BIN/packr
packr version
