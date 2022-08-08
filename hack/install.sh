#!/bin/bash
set -eux -o pipefail

export DOWNLOADS=/tmp/dl
export BIN=${BIN:-/usr/local/bin}

mkdir -p $DOWNLOADS

ARCHITECTURE=""
case $(uname -m) in
    x86_64)                     ARCHITECTURE="amd64" ;;
    arm64)                      ARCHITECTURE="arm64" ;;
    ppc64le)                    ARCHITECTURE="ppc64le" ;;
    s390x)                      ARCHITECTURE="s390x" ;;
    arm|armv7l|armv8l|aarch64)  dpkg --print-architecture | grep -q "arm64" && ARCHITECTURE="arm64" || ARCHITECTURE="arm" ;;
esac

INSTALL_OS=""
unameOut="$(uname -s)"
case "${unameOut}" in
    Linux*)     INSTALL_OS=linux;;
    Darwin*)    INSTALL_OS=darwin;;
esac

for product in $*; do
  ARCHITECTURE=$ARCHITECTURE INSTALL_OS=$INSTALL_OS "$(dirname $0)/installers/install-${product}.sh"
done
