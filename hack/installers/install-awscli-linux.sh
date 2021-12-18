#!/bin/bash
set -eux -o pipefail

. $(dirname $0)/../tool-versions.sh

case $ARCHITECTURE in
  arm64|arm|aarch64)
  export TARGET_FILE=awscli-linux-${ARCHITECTURE}-v${awscliv2_version}.zip
  [ -e ${DOWNLOADS}/${TARGET_FILE} ] || curl -sLf --retry 3 -o ${DOWNLOADS}/${TARGET_FILE} https://awscli.amazonaws.com/awscli-exe-linux-aarch64-$awscliv2_version.zip
  $(dirname $0)/compare-chksum.sh
  mkdir -p /tmp/awscliv2 && unzip $DOWNLOADS/${TARGET_FILE} -d /tmp/awscliv2
  sudo /tmp/awscliv2/aws/install
  aws --version
  ;;
  *)
  export TARGET_FILE=awscli-linux-${ARCHITECTURE}-v${awscliv2_version}.zip
  [ -e ${DOWNLOADS}/${TARGET_FILE} ] || curl -sLf --retry 3 -o ${DOWNLOADS}/${TARGET_FILE} https://awscli.amazonaws.com/awscli-exe-linux-x86_64-$awscliv2_version.zip
  $(dirname $0)/compare-chksum.sh
  mkdir -p /tmp/awscliv2 && unzip $DOWNLOADS/${TARGET_FILE} -d /tmp/awscliv2
  sudo /tmp/awscliv2/aws/install
  aws --version
  ;;
esac
