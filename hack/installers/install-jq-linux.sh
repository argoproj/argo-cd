#!/bin/bash
set -eux -o pipefail

. $(dirname $0)/../tool-versions.sh

case $ARCHITECTURE in
  arm64)
    export TARGET_FILE=jq_${jq_version}-1_${ARCHITECTURE}.deb
    [ -e $DOWNLOADS/${TARGET_FILE} ] || curl -sLf --retry 3 -o $DOWNLOADS/${TARGET_FILE} http://ports.ubuntu.com/ubuntu-ports/pool/universe/j/jq/jq_${jq_version}-1_${ARCHITECTURE}.deb
    $(dirname $0)/compare-chksum.sh
    sudo dpkg --fsys-tarfile $DOWNLOADS/${TARGET_FILE} | tar xOf - ./usr/bin/jq > $BIN/jq
    ;;
  arm)
    ARCHITECTURE=armhf
    export TARGET_FILE=jq_${jq_version}-1_${ARCHITECTURE}.deb
    [ -e $DOWNLOADS/${TARGET_FILE} ] || curl -sLf --retry 3 -o $DOWNLOADS/${TARGET_FILE} http://ports.ubuntu.com/ubuntu-ports/pool/universe/j/jq/jq_${jq_version}-1_${ARCHITECTURE}.deb
    $(dirname $0)/compare-chksum.sh
    sudo dpkg --fsys-tarfile ${TARGET_FILE} | tar xOf - ./usr/bin/jq > $BIN/jq
    ;;
  *)
    export TARGET_FILE=jq-${jq_version}-linux-amd64

    [ -e $DOWNLOADS/${TARGET_FILE} ] || curl -sLf --retry 3 -o $DOWNLOADS/${TARGET_FILE} https://github.com/stedolan/jq/releases/download/jq-${jq_version}/jq-linux64
    $(dirname $0)/compare-chksum.sh
    sudo install -m 0755 $DOWNLOADS/${TARGET_FILE} $BIN/jq
    jq --version
    ;;
esac