#!/bin/bash
set -eux -o pipefail

SRCROOT="$( CDPATH='' cd -- "$(dirname "$0")/.." && pwd -P )"
DEFAULT_BIN="${SRCROOT}/dist"
export BIN=${BIN:-$DEFAULT_BIN}
export DOWNLOADS=/tmp/dl

mkdir -p $DOWNLOADS

ARCHITECTURE=""
case $(uname -m) in
    x86_64)                     ARCHITECTURE="amd64" ;;
    arm64)                      ARCHITECTURE="arm64" ;;
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
