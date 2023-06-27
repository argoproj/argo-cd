#!/bin/bash
set -eux -o pipefail

. $(dirname $0)/../tool-versions.sh

PROJECT_ROOT=$(cd $(dirname ${BASH_SOURCE})/../..; pwd)
INSTALL_PATH="${INSTALL_PATH:-$PROJECT_ROOT/dist}"
PATH="${INSTALL_PATH}:${PATH}"
[ -d $INSTALL_PATH ] || mkdir -p $INSTALL_PATH

KUSTOMIZE_VERSION=${KUSTOMIZE_VERSION:-$kustomize5_version}

if [ -z $INSTALL_OS ]; then
    echo "install kustomize error: unsupported operating system"
    exit 1
fi

# Note that kustomize release URIs have changed for v3.2.1. Then again for
# v3.3.0. When upgrading to versions >= v3.3.0 please change the URI format. And
# also note that as of version v3.3.0, assets are in .tar.gz form.
# v3.2.0 = https://github.com/kubernetes-sigs/kustomize/releases/download/v3.2.0/kustomize_3.2.0_linux_amd64
# v3.2.1 = https://github.com/kubernetes-sigs/kustomize/releases/download/kustomize/v3.2.1/kustomize_kustomize.v3.2.1_linux_amd64
# v3.3.0 = https://github.com/kubernetes-sigs/kustomize/releases/download/kustomize/v3.3.0/kustomize_v3.3.0_linux_amd64.tar.gz
case $ARCHITECTURE in
  arm|arm64|s390x|ppc64le)
      export TARGET_FILE=kustomize_${KUSTOMIZE_VERSION}_${INSTALL_OS}_${ARCHITECTURE}.tar.gz
      URL=https://github.com/kubernetes-sigs/kustomize/releases/download/kustomize/v${KUSTOMIZE_VERSION}/kustomize_v${KUSTOMIZE_VERSION}_${INSTALL_OS}_$ARCHITECTURE.tar.gz
      BINNAME=kustomize
      [ -e ${DOWNLOADS}/${TARGET_FILE} ] || curl -sLf --retry 3 -o ${DOWNLOADS}/${TARGET_FILE} "$URL"
      $(dirname $0)/compare-chksum.sh
      tar -C /tmp -xf ${DOWNLOADS}/${TARGET_FILE}
      sudo install -m 0755 /tmp/kustomize $INSTALL_PATH/$BINNAME
      ;;
  *)
    case $KUSTOMIZE_VERSION in
      2.*)
        export TARGET_FILE=kustomize_${KUSTOMIZE_VERSION}_${INSTALL_OS}_${ARCHITECTURE}
        URL=https://github.com/kubernetes-sigs/kustomize/releases/download/v${KUSTOMIZE_VERSION}/kustomize_${KUSTOMIZE_VERSION}_${INSTALL_OS}_$ARCHITECTURE
        BINNAME=kustomize2
        [ -e ${DOWNLOADS}/${TARGET_FILE} ] || curl -sLf --retry 3 -o ${DOWNLOADS}/${TARGET_FILE} "$URL"
        $(dirname $0)/compare-chksum.sh
        sudo install -m 0755 ${DOWNLOADS}/${TARGET_FILE} $INSTALL_PATH/$BINNAME
        ;;
      *)
        export TARGET_FILE=kustomize_${KUSTOMIZE_VERSION}_${INSTALL_OS}_${ARCHITECTURE}.tar.gz
        URL=https://github.com/kubernetes-sigs/kustomize/releases/download/kustomize/v${KUSTOMIZE_VERSION}/kustomize_v${KUSTOMIZE_VERSION}_${INSTALL_OS}_$ARCHITECTURE.tar.gz
        BINNAME=kustomize
        [ -e ${DOWNLOADS}/${TARGET_FILE} ] || curl -sLf --retry 3 -o ${DOWNLOADS}/${TARGET_FILE} "$URL"
        $(dirname $0)/compare-chksum.sh
        tar -C /tmp -xf ${DOWNLOADS}/${TARGET_FILE}
        sudo install -m 0755 /tmp/kustomize $INSTALL_PATH/$BINNAME
        ;;
    esac
    ;;
esac

$BINNAME version
